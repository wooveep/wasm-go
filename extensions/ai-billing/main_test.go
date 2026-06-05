package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
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

var billingConfigWithoutPath = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":     "global",
		"provider":        "openai",
		"tenant_header":   "x-tenant-id",
		"consumer_header": "x-consumer-id",
		"billing_service": map[string]interface{}{
			"service_name": "billing.static",
			"service_port": 8080,
			"timeout":      750,
			"auth_token":   "<shared-secret>",
		},
	})
	return data
}()

func mustBillingConfig(t *testing.T, value map[string]interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

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

		t.Run("billing service path defaults to internal endpoint", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfigWithoutPath)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "/internal/billing/events", billingConfig.BillingService.Path)
		})

		t.Run("default consumer header", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfigDefaultConsumer)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "<shared-secret>", billingConfig.BillingService.AuthToken)
			require.Equal(t, defaultConsumerHeader, billingConfig.ConsumerHeader)
		})

		t.Run("global billing service with defaultable fields", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing.static", billingConfig.BillingService.ServiceName)
			require.Equal(t, 80, billingConfig.BillingService.ServicePort)
			require.Equal(t, "/internal/billing/events", billingConfig.BillingService.Path)
			require.Equal(t, defaultTimeout, billingConfig.BillingService.Timeout)
			require.Equal(t, defaultQuotaScope, billingConfig.QuotaScope)
			require.Equal(t, defaultProvider, billingConfig.Provider)
			require.Equal(t, defaultTenantHeader, billingConfig.TenantHeader)
			require.Equal(t, defaultConsumerHeader, billingConfig.ConsumerHeader)
			require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("partial rule inherits billing service and overrides selected fields", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"quota_scope":          "global-scope",
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"fail_policy":          FailPolicyOpen,
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
					"service_port": 8080,
					"path":         "/internal/billing/events",
					"timeout":      750,
					"auth_token":   "<shared-secret>",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-provider"},
						"provider":      "anthropic",
						"quota_scope":   "route-scope",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-provider"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing.static", billingConfig.BillingService.ServiceName)
			require.Equal(t, 8080, billingConfig.BillingService.ServicePort)
			require.Equal(t, "/internal/billing/events", billingConfig.BillingService.Path)
			require.Equal(t, uint32(750), billingConfig.BillingService.Timeout)
			require.Equal(t, "<shared-secret>", billingConfig.BillingService.AuthToken)
			require.Equal(t, "route-scope", billingConfig.QuotaScope)
			require.Equal(t, "anthropic", billingConfig.Provider)
			require.Equal(t, "x-tenant-id", billingConfig.TenantHeader)
			require.Equal(t, "x-consumer-id", billingConfig.ConsumerHeader)
			require.Equal(t, []string{"/v1/chat/completions"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("partial rule only overrides provider and inherits billing service", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"quota_scope":          "global-scope",
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"fail_policy":          FailPolicyOpen,
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
					"service_port": 8080,
					"path":         "/internal/billing/events",
					"timeout":      750,
					"auth_token":   "<shared-secret>",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-provider-only"},
						"provider":      "anthropic",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-provider-only"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing.static", billingConfig.BillingService.ServiceName)
			require.Equal(t, 8080, billingConfig.BillingService.ServicePort)
			require.Equal(t, "/internal/billing/events", billingConfig.BillingService.Path)
			require.Equal(t, uint32(750), billingConfig.BillingService.Timeout)
			require.Equal(t, "<shared-secret>", billingConfig.BillingService.AuthToken)
			require.Equal(t, "global-scope", billingConfig.QuotaScope)
			require.Equal(t, "anthropic", billingConfig.Provider)
			require.Equal(t, "x-tenant-id", billingConfig.TenantHeader)
			require.Equal(t, "x-consumer-id", billingConfig.ConsumerHeader)
			require.Equal(t, []string{"/v1/chat/completions"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("partial rule overrides path suffixes and inherits fail policy", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider":             "openai",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"fail_policy":          FailPolicyOpen,
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_":        []string{"route-custom-path"},
						"enable_path_suffixes": []string{"/custom/ai"},
						"fail_policy":          FailPolicyOpen,
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-custom-path"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "openai", billingConfig.Provider)
			require.Equal(t, []string{"/custom/ai"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("unsupported rule fail policy fails rule parsing", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider":    "openai",
				"fail_policy": FailPolicyOpen,
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-invalid-fail-policy"},
						"fail_policy":   "close",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("full rule config remains valid and can override billing service", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider": "openai",
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-full"},
						"provider":      "dashscope",
						"billing_service": map[string]interface{}{
							"service_name": "billing.route",
							"service_port": 9090,
							"path":         "/route/events",
							"timeout":      900,
							"auth_token":   "<route-secret>",
						},
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-full"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing.route", billingConfig.BillingService.ServiceName)
			require.Equal(t, 9090, billingConfig.BillingService.ServicePort)
			require.Equal(t, "/route/events", billingConfig.BillingService.Path)
			require.Equal(t, uint32(900), billingConfig.BillingService.Timeout)
			require.Equal(t, "<route-secret>", billingConfig.BillingService.AuthToken)
			require.Equal(t, "dashscope", billingConfig.Provider)
		})

		t.Run("missing global billing service fails even when rule has billing service", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider": "openai",
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-full"},
						"billing_service": map[string]interface{}{
							"service_name": "billing.route",
						},
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
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

			action = host.CallOnHttpStreamingRequestBody([]byte(`{"secret":"do-not-buffer"}`), false)
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
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			requireObjectFactName(t, event, "provider", "openai")
			requireObjectFactName(t, event, "model", "gpt-4")
			requireObjectFactName(t, event, "route", "route-a")
			require.Equal(t, "cluster-a", event["cluster"])
			require.Equal(t, "/v1/chat/completions", event["request_path"])
			require.EqualValues(t, 200, event["status_code"])
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "token", usage["unit"])
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.Equal(t, map[string]interface{}{
				"provider_usage": map[string]interface{}{
					"prompt_tokens":     float64(5),
					"completion_tokens": float64(8),
					"total_tokens":      float64(13),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, "provider", event["usage_source"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "pv-7", event["price_version"])
			require.Equal(t, "global", event["quota_scope"])
			require.NotContains(t, event, "input_tokens")
			require.NotContains(t, event, "output_tokens")
			require.NotContains(t, event, "total_tokens")
			require.NotContains(t, event, "gateway_calculated_cost")

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("provider usage without supported cache fields emits only basic usage details", func(t *testing.T) {
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
			action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"audio_tokens":2},"completion_tokens":8,"completion_tokens_details":{"reasoning_tokens":3},"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.NotContains(t, usage, "input_cache_hit_tokens")
			require.NotContains(t, usage, "input_cache_miss_tokens")
			require.NotContains(t, usage, "output_tokens")
			require.Equal(t, map[string]interface{}{
				"provider_usage": map[string]interface{}{
					"prompt_tokens": float64(5),
					"prompt_tokens_details": map[string]interface{}{
						"audio_tokens": float64(2),
					},
					"completion_tokens": float64(8),
					"completion_tokens_details": map[string]interface{}{
						"reasoning_tokens": float64(3),
					},
					"total_tokens": float64(13),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceProvider, event["usage_source"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("matched rules emit resolved provider and quota scope", func(t *testing.T) {
			config := mustBillingConfig(t, map[string]interface{}{
				"quota_scope":          "global-scope",
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
					"service_port": 8080,
					"path":         "/internal/billing/events",
					"timeout":      750,
					"auth_token":   "<shared-secret>",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-openai"},
						"provider":      "openai-route",
					},
					{
						"_match_route_": []string{"route-anthropic"},
						"provider":      "anthropic-route",
						"quota_scope":   "route-scope",
					},
				},
			})

			openaiEvent := deliverTestBillingEvent(t, config, "route-openai", "req-openai")
			requireObjectFactName(t, openaiEvent, "provider", "openai-route")
			require.Equal(t, "global-scope", openaiEvent["quota_scope"])

			anthropicEvent := deliverTestBillingEvent(t, config, "route-anthropic", "req-anthropic")
			requireObjectFactName(t, anthropicEvent, "provider", "anthropic-route")
			require.Equal(t, "route-scope", anthropicEvent["quota_scope"])
		})

		t.Run("full rule config uses rule billing service at runtime", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"billing_service": map[string]interface{}{
					"service_name": "billing.static",
					"service_port": 8080,
					"path":         "/internal/billing/events",
					"timeout":      750,
					"auth_token":   "<shared-secret>",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-full-runtime"},
						"provider":      "dashscope",
						"billing_service": map[string]interface{}{
							"service_name": "billing.route",
							"service_port": 9090,
							"path":         "/route/events",
							"timeout":      900,
							"auth_token":   "<route-secret>",
						},
					},
				},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-full-runtime"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-route-service"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(`{"model":"qwen-plus","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			require.Equal(t, "outbound|9090||billing.route", attrs[0].Upstream)
			require.Contains(t, attrs[0].Headers, [2]string{"Authorization", "Bearer <route-secret>"})
			require.Contains(t, attrs[0].Headers, [2]string{":path", "/route/events"})
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			requireObjectFactName(t, event, "provider", "dashscope")

			host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
			host.CompleteHttp()
		})

		t.Run("each ai request gets new event identity", func(t *testing.T) {
			buildEvent := func(requestID string) map[string]interface{} {
				host, status := test.NewTestHost(billingConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-request-id", requestID},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				})
				host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))

				attrs := host.GetHttpCalloutAttributes()
				require.Len(t, attrs, 1)
				var event map[string]interface{}
				require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
				eventID, ok := event["event_id"].(string)
				require.True(t, ok)
				require.NotEmpty(t, eventID)
				require.Equal(t, eventID, event["idempotency_key"])
				parsed, err := uuid.Parse(eventID)
				require.NoError(t, err)
				require.Equal(t, uuid.Version(7), parsed.Version())
				return event
			}

			events := make([]map[string]interface{}, 0, 2)
			for i := 0; i < 2; i++ {
				events = append(events, buildEvent("req-"+strconv.Itoa(i+1)))
			}
			require.NotEqual(t, events[0]["event_id"], events[1]["event_id"])
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
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer <raw-api-key>"},
				{"x-api-key", "<raw-api-key>"},
				{"x-user-id", "user-id-sample"},
				{"x-api-key-id", "<api-key-id>"},
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
			require.Equal(t, "tenant-a", event["tenant"])
			require.NotContains(t, body, "user-id-sample")
			require.NotContains(t, body, "<raw-api-key>")
			require.NotContains(t, body, "<api-key-id>")

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

		t.Run("streaming cache usage maps explicit cache aware usage", func(t *testing.T) {
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
				{"content-type", "text/event-stream"},
			})
			action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":9,\"prompt_tokens_details\":{\"cached_tokens\":4},\"completion_tokens\":6,\"total_tokens\":15}}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.Equal(t, true, event["is_stream"])
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 9, usage["input"])
			require.EqualValues(t, 6, usage["output"])
			require.EqualValues(t, 15, usage["total"])
			require.EqualValues(t, 4, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 5, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 6, usage["output_tokens"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("kimi top-level cached tokens map cache aware usage", func(t *testing.T) {
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
			action := host.CallOnHttpResponseBody([]byte(`{"model":"moonshot-v1","usage":{"prompt_tokens":12,"cached_tokens":5,"completion_tokens":6,"total_tokens":18}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 12, usage["input"])
			require.EqualValues(t, 6, usage["output"])
			require.EqualValues(t, 18, usage["total"])
			require.EqualValues(t, 5, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 7, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 6, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(5),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens":     float64(12),
					"cached_tokens":     float64(5),
					"completion_tokens": float64(6),
					"total_tokens":      float64(18),
				},
			}, usage["details"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("cached tokens clamp to provider input tokens", func(t *testing.T) {
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
			action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":8},"completion_tokens":2,"total_tokens":7}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 5, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 0, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 2, usage["output_tokens"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("deepseek explicit cache split maps hit and miss usage", func(t *testing.T) {
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
			action := host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-chat","usage":{"prompt_tokens":11,"prompt_cache_hit_tokens":4,"prompt_cache_miss_tokens":7,"completion_tokens":2,"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 11, usage["input"])
			require.EqualValues(t, 2, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.EqualValues(t, 4, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 7, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 2, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"prompt_cache_hit_tokens":  float64(4),
					"prompt_cache_miss_tokens": float64(7),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens":            float64(11),
					"prompt_cache_hit_tokens":  float64(4),
					"prompt_cache_miss_tokens": float64(7),
					"completion_tokens":        float64(2),
					"total_tokens":             float64(13),
				},
			}, usage["details"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("streaming provider usage details merge across chunks", func(t *testing.T) {
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
				{"content-type", "text/event-stream"},
			})

			action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":5,\"prompt_tokens_details\":{\"cached_tokens\":2}}}\n\n"), false)
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"completion_tokens\":8,\"total_tokens\":13,\"prompt_tokens_details\":{\"audio_tokens\":1},\"completion_tokens_details\":{\"reasoning_tokens\":3}}}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			details, ok := usage["details"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(2),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens": float64(5),
					"prompt_tokens_details": map[string]interface{}{
						"cached_tokens": float64(2),
						"audio_tokens":  float64(1),
					},
					"completion_tokens": float64(8),
					"total_tokens":      float64(13),
					"completion_tokens_details": map[string]interface{}{
						"reasoning_tokens": float64(3),
					},
				},
			}, details)

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
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
			require.Equal(t, "missing", event["usage_source"])

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

		t.Run("request text is stored for estimation without raw payload details", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer sk-sensitive"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello input"}],"metadata":{"api_key":"sk-sensitive","trace":"raw-secret"}}`))
			require.Equal(t, types.ActionContinue, action)
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4o-mini","choices":[{"message":{"content":"hello output"}}]}`))

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			require.NotContains(t, string(attrs[0].Body), "sk-sensitive")
			require.NotContains(t, string(attrs[0].Body), "raw-secret")
			require.NotContains(t, string(attrs[0].Body), "messages")

			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceEstimated, event["usage_source"])
			requireObjectFactName(t, event, "model", "gpt-4o-mini")
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.ElementsMatch(t, []string{"unit", "input", "output", "total"}, mapKeys(usage))
			require.Greater(t, int64(usage["input"].(float64)), int64(0))
			require.Greater(t, int64(usage["output"].(float64)), int64(0))

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
			require.EqualValues(t, 2, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 3, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 8, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(2),
				},
				"output": map[string]interface{}{
					"reasoning_tokens": float64(3),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens": float64(5),
					"prompt_tokens_details": map[string]interface{}{
						"cached_tokens": float64(2),
					},
					"completion_tokens": float64(8),
					"completion_tokens_details": map[string]interface{}{
						"reasoning_tokens": float64(3),
					},
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("anthropic cache usage maps explicit cache aware usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/messages"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"message":{"model":"claude-3-5-sonnet","usage":{"input_tokens":10,"output_tokens":7}},"usage":{"cache_creation_input_tokens":4,"cache_read_input_tokens":3}}`))

			attrs := host.GetHttpCalloutAttributes()
			require.Len(t, attrs, 1)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(attrs[0].Body, &event))

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 17, usage["input"])
			require.EqualValues(t, 7, usage["output"])
			require.EqualValues(t, 24, usage["total"])
			require.EqualValues(t, 3, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 14, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 7, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cache_creation_input_tokens": float64(4),
					"cache_read_input_tokens":     float64(3),
				},
				"provider_usage": map[string]interface{}{
					"input_tokens":                float64(10),
					"output_tokens":               float64(7),
					"cache_creation_input_tokens": float64(4),
					"cache_read_input_tokens":     float64(3),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])

			host.CallOnHttpCall([][2]string{{":status", "202"}}, nil)
			host.CompleteHttp()
		})

		t.Run("delivery failures are fail open", func(t *testing.T) {
			cases := []struct {
				name       string
				statusCode int
			}{
				{name: "unauthorized", statusCode: http.StatusUnauthorized},
				{name: "forbidden", statusCode: http.StatusForbidden},
				{name: "request timeout", statusCode: http.StatusRequestTimeout},
				{name: "too many requests", statusCode: http.StatusTooManyRequests},
				{name: "internal server error", statusCode: http.StatusInternalServerError},
				{name: "bad gateway", statusCode: http.StatusBadGateway},
				{name: "service unavailable", statusCode: http.StatusServiceUnavailable},
				{name: "gateway timeout", statusCode: http.StatusGatewayTimeout},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					require.False(t, isBillingDeliveryAcceptedStatus(tc.statusCode), "status %d", tc.statusCode)
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
					action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
					require.Equal(t, types.ActionContinue, action)
					require.Len(t, host.GetHttpCalloutAttributes(), 1)

					host.CallOnHttpCall([][2]string{{":status", strconv.Itoa(tc.statusCode)}}, []byte(`{"error":"temporary"}`))
					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
					host.CompleteHttp()
				})
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

func deliverTestBillingEvent(t *testing.T, config json.RawMessage, routeName, requestID string) map[string]interface{} {
	t.Helper()
	host, status := test.NewTestHost(config)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName(routeName))

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-request-id", requestID},
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
	host.CallOnHttpCall([][2]string{{":status", "202"}}, []byte(`{"ok":true}`))
	host.CompleteHttp()
	return event
}

func requireObjectFactName(t *testing.T, event map[string]interface{}, key, name string) {
	t.Helper()
	value, ok := event[key].(map[string]interface{})
	require.True(t, ok, "%s must be serialized as an object", key)
	require.Equal(t, name, value["name"])
	_, hasID := value["id"]
	require.False(t, hasID, "%s.id should be omitted when no stable Console id is available", key)
}

func mapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestPathFiltering(t *testing.T) {
	require.True(t, isAIPathEnabled("/proxy/v1/chat/completions?x=1", []string{"/v1/chat/completions"}))
	require.False(t, isAIPathEnabled("/proxy/not-ai", []string{"/v1/chat/completions"}))
}

func TestStatusCodeFromHeaders(t *testing.T) {
	require.Equal(t, http.StatusAccepted, statusCodeFromHeaders([][2]string{{":status", "202"}}))
	require.Equal(t, http.StatusBadGateway, statusCodeFromHeaders(nil))
}

func TestBillingDeliveryFailureStatusClassification(t *testing.T) {
	failureStatuses := []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
	for _, statusCode := range failureStatuses {
		require.True(t, isBillingDeliveryFailureStatus(statusCode), "status %d", statusCode)
	}

	acceptedStatuses := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusBadRequest,
		http.StatusNotFound,
	}
	for _, statusCode := range acceptedStatuses {
		require.False(t, isBillingDeliveryFailureStatus(statusCode), "status %d", statusCode)
	}
}

func TestBillingDeliveryAcceptedStatusExcludesAuthFailures(t *testing.T) {
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		require.False(t, isBillingDeliveryAcceptedStatus(statusCode), "status %d", statusCode)
	}

	for _, statusCode := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		require.True(t, isBillingDeliveryAcceptedStatus(statusCode), "status %d", statusCode)
	}
}

func TestCacheAwareInputTokenSplit(t *testing.T) {
	tests := []struct {
		name       string
		input      int64
		details    map[string]int64
		wantHit    int64
		wantMiss   int64
		wantMapped bool
	}{
		{
			name:  "cached tokens clamp to input",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyCachedTokens: 12,
			},
			wantHit:    10,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:  "negative cached tokens become zero",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyCachedTokens: -2,
			},
			wantHit:    0,
			wantMiss:   10,
			wantMapped: true,
		},
		{
			name:  "gemini cached content clamps to input",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyGeminiCachedContentTokenCount: 12,
			},
			wantHit:    10,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:  "deepseek explicit split preserves provider hit and miss",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:  8,
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: 7,
			},
			wantHit:    8,
			wantMiss:   7,
			wantMapped: true,
		},
		{
			name:  "deepseek negative explicit split becomes zero",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:  -2,
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: -3,
			},
			wantHit:    0,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:  "deepseek miss only preserves provider miss",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: 12,
			},
			wantHit:    0,
			wantMiss:   12,
			wantMapped: true,
		},
		{
			name:  "anthropic negative cache values become zero",
			input: -10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens: -4,
				tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens:     -3,
			},
			wantHit:    0,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:       "unsupported details do not map cache split",
			input:      10,
			details:    map[string]int64{"audio_tokens": 2},
			wantMapped: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hitTokens, missTokens, ok := cacheAwareInputTokenSplit(tc.input, tc.details)

			require.Equal(t, tc.wantMapped, ok)
			require.EqualValues(t, tc.wantHit, hitTokens)
			require.EqualValues(t, tc.wantMiss, missTokens)
		})
	}
}

func TestDeliverBillingEventDispatchErrorIsFailOpen(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(billingConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		client := &failingBillingHTTPClient{err: errors.New("network unavailable")}
		ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
		eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "req-dispatch", "tenant-a", "consumer-a", "openai", "global", "")
		require.NoError(t, err)
		ctx.SetContext(ctxEventID, eventID)
		ctx.SetContext(ctxStatusCode, http.StatusOK)
		ctx.SetContext(ctxInputToken, int64(1))
		ctx.SetContext(ctxOutputToken, int64(1))
		ctx.SetContext(ctxTotalToken, int64(2))

		deliverBillingEvent(ctx, BillingConfig{
			Provider: "openai",
			BillingService: BillingService{
				Path:      "/internal/billing/events",
				Timeout:   750,
				AuthToken: "<shared-secret>",
			},
			httpClient: client,
		}, false)

		require.True(t, client.called)
		require.True(t, ctx.GetBoolContext(ctxBillingDelivered, false))
	})
}

func TestSendBillingEventReusesEventIdempotencyKey(t *testing.T) {
	client := &recordingBillingHTTPClient{}
	event := BillingEvent{
		EventID:        "018f4c7c-1111-7abc-8111-111111111111",
		IdempotencyKey: "018f4c7c-1111-7abc-8111-111111111111",
		RequestID:      "req-retry",
		Consumer:       "consumer-a",
		Route:          BillingFact{Name: "route-a"},
		Provider:       BillingFact{Name: "openai"},
		Model:          BillingFact{Name: "gpt-4"},
		RequestPath:    "/v1/chat/completions",
		StatusCode:     http.StatusOK,
		Usage: BillingUsage{
			Unit:    "token",
			Input:   1,
			Output:  2,
			Total:   3,
			Details: map[string]any{},
		},
		StartTimeMs: 1,
		EndTimeMs:   2,
		Cluster:     "cluster-a",
	}
	config := BillingConfig{
		BillingService: BillingService{
			Path:      "/internal/billing/events",
			Timeout:   750,
			AuthToken: "<shared-secret>",
		},
		httpClient: client,
	}

	sendBillingEvent(config, event)
	sendBillingEvent(config, event)

	require.Len(t, client.bodies, 2)
	require.Equal(t, []string{"/internal/billing/events", "/internal/billing/events"}, client.paths)
	require.Len(t, client.headers, 2)
	for _, body := range client.bodies {
		var sent BillingEvent
		require.NoError(t, json.Unmarshal(body, &sent))
		require.Equal(t, event.IdempotencyKey, sent.IdempotencyKey)
		require.Equal(t, event.EventID, sent.EventID)
	}
	for _, headers := range client.headers {
		require.Contains(t, headers, [2]string{"content-type", "application/json"})
		require.Contains(t, headers, [2]string{"Authorization", "Bearer <shared-secret>"})
	}
}

func TestInitBillingRequestContextSetsEventID(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}

	eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "req-1", "tenant-a", "consumer-a", "openai", "global", "pv-7")

	require.NoError(t, err)
	require.NotEmpty(t, eventID)
	require.Equal(t, eventID, ctx.values[ctxEventID])
	require.Equal(t, eventID, ctx.values[ctxIdempotencyKey])
	require.Equal(t, "openai", ctx.values[ctxProvider])
	require.Equal(t, "global", ctx.values[ctxQuotaScope])
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
	eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "", "tenant-a", "consumer-a", "openai", "global", "pv-7")
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

func TestBuildBillingEventEstimatedUsageIsBasicOnly(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxStartTime, int64(1))
	ctx.SetContext(ctxEventID, "018f4c7c-2222-7abc-8222-222222222222")
	ctx.SetContext(ctxIdempotencyKey, "018f4c7c-2222-7abc-8222-222222222222")
	ctx.SetContext(ctxRequestPath, "/v1/chat/completions")
	ctx.SetContext(ctxRequestID, "req-estimated")
	ctx.SetContext(ctxTenant, "tenant-a")
	ctx.SetContext(ctxConsumer, "consumer-a")
	ctx.SetContext(ctxProvider, "openai")
	ctx.SetContext(ctxQuotaScope, "global")
	ctx.SetContext(ctxRoute, "route-a")
	ctx.SetContext(ctxCluster, "cluster-a")
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	require.True(t, recordEstimatedUsage(ctx, "provider-new-model", "hello input", "hello output"))
	ctx.SetContext(ctxInputCacheHit, int64(1))
	ctx.SetContext(ctxInputCacheMiss, int64(2))
	ctx.SetContext(ctxInputDetails, map[string]int64{"cached_tokens": 1})
	ctx.SetContext(ctxOutputDetails, map[string]int64{"reasoning_tokens": 2})
	ctx.SetContext(ctxProviderUsage, map[string]any{"prompt_tokens": 99})

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Equal(t, false, event["usage_missing"])
	require.Equal(t, usageSourceEstimated, event["usage_source"])
	requireObjectFactName(t, event, "model", "provider-new-model")

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "token", usage["unit"])
	input := int64(usage["input"].(float64))
	output := int64(usage["output"].(float64))
	total := int64(usage["total"].(float64))
	require.Greater(t, input, int64(0))
	require.Greater(t, output, int64(0))
	require.Equal(t, input+output, total)
	require.ElementsMatch(t, []string{"unit", "input", "output", "total"}, mapKeys(usage))
}

func TestBuildBillingEventMissingUsageFallbackIsZero(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxStartTime, int64(1))
	ctx.SetContext(ctxEventID, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxIdempotencyKey, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxRequestPath, "/v1/chat/completions")
	ctx.SetContext(ctxRequestID, "req-missing")
	ctx.SetContext(ctxConsumer, "consumer-a")
	ctx.SetContext(ctxProvider, "openai")
	ctx.SetContext(ctxQuotaScope, "global")
	ctx.SetContext(ctxRoute, "route-a")
	ctx.SetContext(ctxCluster, "cluster-a")
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	ctx.SetContext(ctxInputToken, int64(12))
	ctx.SetContext(ctxOutputToken, int64(8))
	ctx.SetContext(ctxInputCacheHit, int64(1))
	ctx.SetContext(ctxInputCacheMiss, int64(2))
	ctx.SetContext(ctxInputDetails, map[string]int64{"cached_tokens": 1})
	ctx.SetContext(ctxProviderUsage, map[string]any{"prompt_tokens": 99})

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Equal(t, true, event["usage_missing"])
	require.Equal(t, usageSourceMissing, event["usage_source"])

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "token", usage["unit"])
	require.EqualValues(t, 0, usage["input"])
	require.EqualValues(t, 0, usage["output"])
	require.EqualValues(t, 0, usage["total"])
	require.Equal(t, map[string]interface{}{}, usage["details"])
	require.NotContains(t, usage, "input_cache_hit_tokens")
	require.NotContains(t, usage, "input_cache_miss_tokens")
	require.NotContains(t, usage, "output_tokens")
}

type failingBillingHTTPClient struct {
	err    error
	called bool
}

func (f *failingBillingHTTPClient) Get(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Head(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Options(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Post(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	f.called = true
	return f.err
}
func (f *failingBillingHTTPClient) Put(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Patch(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Delete(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Connect(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Trace(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) Call(method, rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (f *failingBillingHTTPClient) ClusterName() string { return "billing.static" }

type recordingBillingHTTPClient struct {
	paths   []string
	headers [][][2]string
	bodies  [][]byte
}

func (r *recordingBillingHTTPClient) Get(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Head(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Options(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Post(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	r.paths = append(r.paths, rawURL)
	r.headers = append(r.headers, append([][2]string(nil), headers...))
	r.bodies = append(r.bodies, append([]byte(nil), body...))
	return nil
}
func (r *recordingBillingHTTPClient) Put(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Patch(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Delete(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Connect(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Trace(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) Call(method, rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}
func (r *recordingBillingHTTPClient) ClusterName() string { return "billing.static" }

func TestOnHttpStreamingResponseBodyAccumulatesSentAssistantDeltas(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{
		ctxBillingEnabled: true,
	}}

	firstChunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n")
	returned := onHttpStreamingResponseBody(ctx, BillingConfig{}, firstChunk, false)
	require.Equal(t, firstChunk, returned)

	secondChunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
	returned = onHttpStreamingResponseBody(ctx, BillingConfig{}, secondChunk, false)
	require.Equal(t, secondChunk, returned)

	require.True(t, ctx.GetBoolContext(ctxIsStream, false))
	require.Equal(t, "Hello", ctx.GetStringContext(ctxStreamOutputText, ""))
	require.Empty(t, ctx.GetStringContext(ctxUsageSource, ""))
}

func TestOnHttpStreamingResponseBodyEndOfStreamDeliversAndMarksDelivered(t *testing.T) {
	client := &recordingBillingHTTPClient{}
	ctx := &mockBillingHttpContext{values: map[string]interface{}{
		ctxBillingEnabled: true,
		ctxStartTime:      int64(1),
		ctxEventID:        "018f4c7c-3333-7abc-8333-333333333333",
		ctxIdempotencyKey: "018f4c7c-3333-7abc-8333-333333333333",
		ctxRequestPath:    "/v1/chat/completions",
		ctxRequestID:      "req-stream-end",
		ctxTenant:         "tenant-a",
		ctxConsumer:       "consumer-a",
		ctxProvider:       "openai",
		ctxQuotaScope:     "global",
		ctxRoute:          "route-a",
		ctxCluster:        "cluster-a",
		ctxStatusCode:     http.StatusOK,
	}}

	chunk := []byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n")
	returned := onHttpStreamingResponseBody(ctx, BillingConfig{
		Provider: "openai",
		BillingService: BillingService{
			Path:      "/internal/billing/events",
			Timeout:   750,
			AuthToken: "<shared-secret>",
		},
		httpClient: client,
	}, chunk, true)

	require.Equal(t, chunk, returned)
	require.True(t, ctx.GetBoolContext(ctxBillingDelivered, false))
	require.Len(t, client.bodies, 1)

	var event BillingEvent
	require.NoError(t, json.Unmarshal(client.bodies[0], &event))
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceProvider, event.UsageSource)
	require.EqualValues(t, 1, event.Usage.Input)
	require.EqualValues(t, 2, event.Usage.Output)
	require.EqualValues(t, 3, event.Usage.Total)
}

func TestStreamDoneFallbackDeliversEstimatedUsage(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello input"}]}`))
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()

	attrs := host.GetHttpCalloutAttributes()
	require.Len(t, attrs, 1)

	var event BillingEvent
	require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceEstimated, event.UsageSource)
	require.Greater(t, event.Usage.Input, int64(0))
	require.Greater(t, event.Usage.Output, int64(0))
	require.Equal(t, event.Usage.Input+event.Usage.Output, event.Usage.Total)
	require.Nil(t, event.Usage.Details)
}

func TestStreamDoneFallbackEstimatesOnlySentDeltas(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	requestText := "hello input"
	sentText := "Hello"
	unsentText := " unreachable generated continuation with many distinct words and numbers 12345"

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpRequestBody([]byte(`{"messages":[{"role":"user","content":"` + requestText + `"}]}`))
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\""+sentText+"\"}}]}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()

	attrs := host.GetHttpCalloutAttributes()
	require.Len(t, attrs, 1)

	sentUsage, ok := estimateTextTokenUsage("", requestText, sentText)
	require.True(t, ok)
	unsentUsage, ok := estimateTextTokenUsage("", requestText, sentText+unsentText)
	require.True(t, ok)
	require.Greater(t, unsentUsage.OutputToken, sentUsage.OutputToken)

	var event BillingEvent
	require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
	require.Equal(t, usageSourceEstimated, event.UsageSource)
	require.Equal(t, sentUsage.InputToken, event.Usage.Input)
	require.Equal(t, sentUsage.OutputToken, event.Usage.Output)
	require.Equal(t, sentUsage.TotalToken, event.Usage.Total)
	require.NotEqual(t, unsentUsage.OutputToken, event.Usage.Output)
}

func TestStreamDoneFallbackDeliversProviderUsage(t *testing.T) {
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
		{"content-type", "text/event-stream"},
	})
	action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":5,\"total_tokens\":9}}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()

	attrs := host.GetHttpCalloutAttributes()
	require.Len(t, attrs, 1)

	var event BillingEvent
	require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceProvider, event.UsageSource)
	require.EqualValues(t, 4, event.Usage.Input)
	require.EqualValues(t, 5, event.Usage.Output)
	require.EqualValues(t, 9, event.Usage.Total)
	require.Contains(t, event.Usage.Details, "provider_usage")
}

func TestStreamDoneFallbackDeliversMissingUsage(t *testing.T) {
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
		{"content-type", "text/event-stream"},
	})
	action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\\\"city\\\":\\\"Paris\\\"}\"}}]}}]}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()

	attrs := host.GetHttpCalloutAttributes()
	require.Len(t, attrs, 1)

	var event BillingEvent
	require.NoError(t, json.Unmarshal(attrs[0].Body, &event))
	require.True(t, event.IsStream)
	require.True(t, event.UsageMissing)
	require.Equal(t, usageSourceMissing, event.UsageSource)
	require.EqualValues(t, 0, event.Usage.Input)
	require.EqualValues(t, 0, event.Usage.Output)
	require.EqualValues(t, 0, event.Usage.Total)
	require.Empty(t, event.Usage.Details)
}

func TestStreamDoneFallbackSkipsAfterNormalDelivery(t *testing.T) {
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
		{"content-type", "text/event-stream"},
	})
	action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"), true)
	require.Equal(t, types.ActionContinue, action)
	require.Len(t, host.GetHttpCalloutAttributes(), 1)

	host.CompleteHttp()

	require.Len(t, host.GetHttpCalloutAttributes(), 1)
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
