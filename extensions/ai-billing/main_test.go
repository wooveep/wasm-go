package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var billingConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":     "global",
		"provider":        "openai",
		"tenant_header":   "x-tenant-id",
		"consumer_header": "x-consumer-id",
		"billing_service": map[string]interface{}{
			"service_name": "billing.static",
			"service_port": 8080,
			"path":         "/internal/billing/events",
			"timeout":      750,
			"auth_token":   "<shared-secret>",
		},
		"enable_path_suffixes": []string{
			"/v1/chat/completions",
			"/v1/messages",
		},
	})
	return data
}()

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("billing service target and default fail policy", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing.static", billingConfig.BillingService.ServiceName)
			require.Equal(t, 8080, billingConfig.BillingService.ServicePort)
			require.Equal(t, "/internal/billing/events", billingConfig.BillingService.Path)
			require.Equal(t, uint32(750), billingConfig.BillingService.Timeout)
			require.Equal(t, "<shared-secret>", billingConfig.BillingService.AuthToken)
			require.Equal(t, "global", billingConfig.QuotaScope)
			require.Equal(t, "openai", billingConfig.Provider)
			require.Equal(t, "x-tenant-id", billingConfig.TenantHeader)
			require.Equal(t, "x-consumer-id", billingConfig.ConsumerHeader)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})
	})
}

func TestBillingEventDelivery(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("successful event includes request facts", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-a"))
			require.NoError(t, host.SetClusterName("cluster-a"))
			require.NoError(t, host.SetRequestId("req-1"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-1"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"x-ai-price-version", "pv-7"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":5,"completion_tokens":8,"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			require.Equal(t, "outbound|8080||billing.static", attrs[0].Upstream)
			require.Contains(t, string(attrs[0].Body), `"request_id":"req-1"`)
			require.Contains(t, string(attrs[0].Body), `"idempotency_key":"req-1"`)
			require.Contains(t, string(attrs[0].Body), `"tenant":"tenant-a"`)
			require.Contains(t, string(attrs[0].Body), `"consumer":"consumer-a"`)
			require.Contains(t, string(attrs[0].Body), `"quota_scope":"global"`)
			require.Contains(t, string(attrs[0].Body), `"provider":"openai"`)
			require.Contains(t, string(attrs[0].Body), `"model":"gpt-4"`)
			require.Contains(t, string(attrs[0].Body), `"route":"route-a"`)
			require.Contains(t, string(attrs[0].Body), `"cluster":"cluster-a"`)
			require.Contains(t, string(attrs[0].Body), `"request_path":"/v1/chat/completions"`)
			require.Contains(t, string(attrs[0].Body), `"status_code":200`)
			require.Contains(t, string(attrs[0].Body), `"input_tokens":5`)
			require.Contains(t, string(attrs[0].Body), `"output_tokens":8`)
			require.Contains(t, string(attrs[0].Body), `"total_tokens":13`)
			require.Contains(t, string(attrs[0].Body), `"usage_missing":false`)
			require.Contains(t, string(attrs[0].Body), `"price_version":"pv-7"`)

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("usage missing payload is still emitted", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			require.Contains(t, string(attrs[0].Body), `"usage_missing":true`)
			require.Contains(t, string(attrs[0].Body), `"input_tokens":0`)
			require.Contains(t, string(attrs[0].Body), `"output_tokens":0`)
			require.Contains(t, string(attrs[0].Body), `"total_tokens":0`)
			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("timeout and server error are fail open", func(t *testing.T) {
			for _, headers := range [][][2]string{
				nil,
				{{":status", "503"}},
			} {
				host, status := test.NewTestHost(billingConfig)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				})
				action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
				require.Equal(t, types.ActionContinue, action)
				require.Len(t, host.GetHttpCalloutAttributes(), 1)

				host.CallOnHttpCall(headers, []byte(`{"error":"temporary"}`))
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				host.CompleteHttp()
				host.Reset()
			}
		})

		t.Run("billing plugin does not mutate redis balance", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))

			require.Empty(t, host.GetRedisCalloutAttributes())
			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})
	})
}

func TestPathFiltering(t *testing.T) {
	require.True(t, isAIPathEnabled("/proxy/v1/chat/completions?x=1", []string{"/v1/chat/completions"}))
	require.False(t, isAIPathEnabled("/proxy/not-ai", []string{"/v1/chat/completions"}))
}

func TestStatusCodeFromHeaders(t *testing.T) {
	require.Equal(t, http.StatusAccepted, statusCodeFromHeaders([][2]string{{":status", "202"}}))
	require.Equal(t, http.StatusBadGateway, statusCodeFromHeaders(nil))
}
