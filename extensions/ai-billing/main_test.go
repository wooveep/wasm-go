package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/iface"
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

var billingConfigDefaultConsumer = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":   "global",
		"provider":      "openai",
		"tenant_header": "x-tenant-id",
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
		t.Run("non-ai path does not dispatch", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/healthz"},
				{":method", "GET"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"ok":true}`))
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

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
			require.Contains(t, attrs[0].Headers, [2]string{"content-type", "application/json"})
			require.Contains(t, attrs[0].Headers, [2]string{"Authorization", "Bearer <shared-secret>"})
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.NotEmpty(t, event["event_id"])
			require.Equal(t, "req-1", event["request_id"])
			require.NotEmpty(t, event["idempotency_key"])
			require.NotEqual(t, "req-1", event["idempotency_key"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.Equal(t, "openai", event["provider"])
			require.Equal(t, "gpt-4", event["model"])
			require.Equal(t, "route-a", event["route"])
			require.Equal(t, "cluster-a", event["cluster"])
			require.Equal(t, "/v1/chat/completions", event["request_path"])
			require.EqualValues(t, 200, event["status_code"])
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "token", usage["unit"])
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.Equal(t, map[string]interface{}{}, usage["details"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "pv-7", event["price_version"])
			require.NotContains(t, event, "tenant")
			require.NotContains(t, event, "quota_scope")
			require.NotContains(t, event, "input_tokens")
			require.NotContains(t, event, "output_tokens")
			require.NotContains(t, event, "total_tokens")
			require.NotContains(t, event, "gateway_calculated_cost")

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("payload omits credentials and forbidden identity fields", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-secure"))
			require.NoError(t, host.SetClusterName("cluster-secure"))
			require.NoError(t, host.SetRequestId("req-sensitive"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-sensitive"},
				{"x-tenant-id", "tenant-id-should-not-leak"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer sk-live-cred"},
				{"x-api-key", "api-key-sample"},
				{"x-user-id", "user-id-sample"},
				{"x-api-key-id", "api-key-id-sample"},
				{"x-ai-price-version", "pv-secure"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			body := string(attrs[0].Body)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))

			forbiddenFields := []string{
				"tenant_id",
				"user_id",
				"api_key_id",
				"consumer_id",
			}
			for _, key := range forbiddenFields {
				_, hasKey := event[key]
				require.False(t, hasKey)
			}
			require.NotContains(t, body, "tenant-id-should-not-leak")
			require.NotContains(t, body, "user-id-sample")
			require.NotContains(t, body, "api-key-sample")
			require.NotContains(t, body, "api-key-id-sample")
			require.NotContains(t, body, "sk-live-cred")

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("x-request-id header takes precedence over x_request_id property", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("property-request-id"))
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "header-request-id"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.Equal(t, "header-request-id", event["request_id"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("x_request_id property is fallback request_id source when header is absent", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("property-request-id"))
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.Equal(t, "property-request-id", event["request_id"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("default consumer header uses x-mse-consumer", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfigDefaultConsumer)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"X-Mse-Consumer", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.Equal(t, "consumer-a", event["consumer"])
			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("streaming response sets stream state", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-a"))
			require.NoError(t, host.SetClusterName("cluster-a"))
			require.NoError(t, host.SetRequestId("req-2"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-2"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\"}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			require.Contains(t, string(attrs[0].Body), `"is_stream":true`)

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
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.Equal(t, true, event["usage_missing"])

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "token", usage["unit"])
			require.EqualValues(t, 0, usage["input"])
			require.EqualValues(t, 0, usage["output"])
			require.EqualValues(t, 0, usage["total"])
			require.Equal(t, map[string]interface{}{}, usage["details"])
			require.NotContains(t, string(attrs[0].Body), `"input_tokens":`)
			require.NotContains(t, string(attrs[0].Body), `"output_tokens":`)
			require.NotContains(t, string(attrs[0].Body), `"total_tokens":`)
			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("structured token usage maps details and total fallback", func(t *testing.T) {
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
			host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens":8,"completion_tokens_details":{"reasoning_tokens":3}}}`))

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(2),
				},
				"output": map[string]interface{}{
					"reasoning_tokens": float64(3),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])

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

func TestInitBillingRequestContextSetsEventID(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}

	eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "req-1", "tenant-a", "consumer-a", "openai", "pv-7")

	require.NoError(t, err)
	require.NotEmpty(t, eventID)
	require.Equal(t, eventID, ctx.values[ctxEventID])
	require.Equal(t, eventID, ctx.values[ctxIdempotencyKey])
	require.Equal(t, "openai", ctx.values[ctxProvider])
	require.Equal(t, "/v1/chat/completions", ctx.values[ctxRequestPath])
	require.Equal(t, "req-1", ctx.values[ctxRequestID])
	require.Equal(t, "tenant-a", ctx.values[ctxTenant])
	require.Equal(t, "consumer-a", ctx.values[ctxConsumer])
	require.NotZero(t, ctx.values[ctxStartTime])
	parsed, err := uuid.Parse(eventID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())
}

func TestBuildBillingEventUsesOnlyRequestIdSources(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "", "tenant-a", "consumer-a", "openai", "pv-7")
	require.NoError(t, err)

	ctx.SetContext(ctxStatusCode, http.StatusOK)

	event := buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false)
	require.Equal(t, eventID, event.EventID)
	require.Equal(t, "", event.RequestID)
	require.Equal(t, "consumer-a", event.Consumer)
	require.NotEqual(t, event.RequestID, "tenant-a")
	require.NotEqual(t, event.RequestID, "consumer-a")
	require.NotEqual(t, event.RequestID, "200")
}

type mockBillingHttpContext struct {
	values map[string]interface{}
}

func (m *mockBillingHttpContext) Scheme() string { return "" }
func (m *mockBillingHttpContext) Host() string   { return "" }
func (m *mockBillingHttpContext) Path() string   { return "" }
func (m *mockBillingHttpContext) Method() string { return "" }
func (m *mockBillingHttpContext) SetContext(key string, value interface{}) {
	m.values[key] = value
}
func (m *mockBillingHttpContext) GetContext(key string) interface{} { return m.values[key] }
func (m *mockBillingHttpContext) GetBoolContext(key string, defaultValue bool) bool {
	if v, ok := m.values[key].(bool); ok {
		return v
	}
	return defaultValue
}
func (m *mockBillingHttpContext) GetStringContext(key, defaultValue string) string {
	if v, ok := m.values[key].(string); ok {
		return v
	}
	return defaultValue
}
func (m *mockBillingHttpContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	if v, ok := m.values[key].([]byte); ok {
		return v
	}
	return defaultValue
}
func (m *mockBillingHttpContext) GetUserAttribute(key string) interface{} { return nil }
func (m *mockBillingHttpContext) SetUserAttribute(key string, value interface{}) {
}
func (m *mockBillingHttpContext) SetUserAttributeMap(kvmap map[string]interface{}) {}
func (m *mockBillingHttpContext) GetUserAttributeMap() map[string]interface{}      { return nil }
func (m *mockBillingHttpContext) WriteUserAttributeToLog() error                   { return nil }
func (m *mockBillingHttpContext) WriteUserAttributeToLogWithKey(key string) error  { return nil }
func (m *mockBillingHttpContext) WriteUserAttributeToTrace() error                 { return nil }
func (m *mockBillingHttpContext) DontReadRequestBody()                             {}
func (m *mockBillingHttpContext) DontReadResponseBody()                            {}
func (m *mockBillingHttpContext) BufferRequestBody()                               {}
func (m *mockBillingHttpContext) BufferResponseBody()                              {}
func (m *mockBillingHttpContext) NeedPauseStreamingResponse()                      {}
func (m *mockBillingHttpContext) PushBuffer(buffer []byte)                         {}
func (m *mockBillingHttpContext) PopBuffer() []byte                                { return nil }
func (m *mockBillingHttpContext) BufferQueueSize() int                             { return 0 }
func (m *mockBillingHttpContext) DisableReroute()                                  {}
func (m *mockBillingHttpContext) SetRequestBodyBufferLimit(byteSize uint32)        {}
func (m *mockBillingHttpContext) SetResponseBodyBufferLimit(byteSize uint32)       {}
func (m *mockBillingHttpContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (m *mockBillingHttpContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.DecodeHeader
}
func (m *mockBillingHttpContext) HasRequestBody() bool       { return false }
func (m *mockBillingHttpContext) HasResponseBody() bool      { return false }
func (m *mockBillingHttpContext) IsWebsocket() bool          { return false }
func (m *mockBillingHttpContext) IsBinaryRequestBody() bool  { return false }
func (m *mockBillingHttpContext) IsBinaryResponseBody() bool { return false }
