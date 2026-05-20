package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var monetaryConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":            "global",
		"provider":               "openai",
		"tenant_header":          "x-tenant-id",
		"consumer_header":        "x-consumer-id",
		"balance_key_template":   "billing:balance:{tenant}:{quota_scope}:{consumer}",
		"price_key_template":     "billing:effective_price:{tenant}:{provider}:{model}:{token_type}",
		"missing_balance_policy": "deny",
		"missing_price_policy":   "skip",
		"missing_usage_policy":   "skip",
		"enable_path_suffixes": []string{
			"/v1/chat/completions",
			"/v1/messages",
		},
		"redis": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
			"timeout":      1000,
			"database":     0,
		},
	})
	return data
}()

func quotaConfigWith(overrides map[string]interface{}) json.RawMessage {
	var cfg map[string]interface{}
	_ = json.Unmarshal(monetaryConfig, &cfg)
	for k, v := range overrides {
		cfg[k] = v
	}
	data, _ := json.Marshal(cfg)
	return data
}

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("default monetary config", func(t *testing.T) {
			data, _ := json.Marshal(map[string]interface{}{
				"redis": map[string]interface{}{
					"service_name": "redis.static",
				},
			})
			host, status := test.NewTestHost(data)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			quotaConfig := config.(*QuotaConfig)
			require.Equal(t, "global", quotaConfig.QuotaScope)
			require.Equal(t, "default", quotaConfig.Provider)
			require.Equal(t, "x-mse-tenant", quotaConfig.TenantHeader)
			require.Equal(t, "x-mse-consumer", quotaConfig.ConsumerHeader)
			require.Equal(t, "billing:balance:{tenant}:{quota_scope}:{consumer}", quotaConfig.BalanceKeyTemplate)
			require.Equal(t, "billing:effective_price:{tenant}:{provider}:{model}:{token_type}", quotaConfig.PriceKeyTemplate)
			require.Equal(t, int64(1000000), quotaConfig.AmountScale)
			require.Equal(t, int64(1000000), quotaConfig.PriceUnitTokens)
			require.Equal(t, MissingPolicyDeny, quotaConfig.MissingBalancePolicy)
			require.Equal(t, MissingPolicySkip, quotaConfig.MissingPricePolicy)
			require.Equal(t, MissingPolicySkip, quotaConfig.MissingUsagePolicy)
			require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, quotaConfig.EnablePathSuffixes)
		})

		t.Run("admin config no longer required", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
				"admin_consumer": nil,
				"admin_path":     nil,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})
	})
}

func TestRequestAdmission(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("positive balance allows request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			require.Contains(t, string(host.GetRedisCalloutAttributes()[0].Query), "billing:balance:tenant-a:global:consumer-a")

			host.CallOnRedisCall(0, test.CreateRedisRespString("100"))
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("non-positive balance denies request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

			host.CallOnRedisCall(0, test.CreateRedisRespInt(0))
			response := host.GetLocalResponse()
			require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
			require.Contains(t, string(response.Data), "No monetary balance left")
			host.CompleteHttp()
		})

		t.Run("missing balance follows allow policy", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
				"missing_balance_policy": "allow",
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("missing identity denies request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			response := host.GetLocalResponse()
			require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
			require.Contains(t, string(response.Data), "Missing tenant or consumer identity")
		})

		t.Run("legacy admin paths are not handled", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions/quota?consumer=consumer-a"},
				{":method", "GET"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
		})
	})
}

func TestMonetaryDeduction(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("usage and prices produce eval deduction", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnRedisCall(0, test.CreateRedisRespInt(1000))

			responseBody := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":1001,"completion_tokens":2000,"total_tokens":3001}}`)
			action := host.CallOnHttpStreamingResponseBody(responseBody, true)
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetRedisCalloutAttributes()
			require.Len(t, attrs, 1)
			query := string(attrs[0].Query)
			require.Contains(t, query, "eval")
			require.Contains(t, query, "billing:balance:tenant-a:global:consumer-a")
			require.Contains(t, query, "billing:effective_price:tenant-a:openai:gpt-4:input")
			require.Contains(t, query, "billing:effective_price:tenant-a:openai:gpt-4:output")
			require.Contains(t, query, "1001")
			require.Contains(t, query, "2000")
			host.CallOnRedisCall(0, test.CreateRedisRespArray([]interface{}{5, 2, 3}))
			host.CompleteHttp()
		})

		t.Run("missing usage skips deduction", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnRedisCall(0, test.CreateRedisRespInt(1000))

			action := host.CallOnHttpStreamingResponseBody([]byte(`{"choices":[{"message":{"content":"ok"}}]}`), true)
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
			host.CompleteHttp()
		})
	})
}

func TestCostCalculation(t *testing.T) {
	require.Equal(t, int64(5), calculateCost(1001, 2000, 1000, 1500, 1000000))
	require.Equal(t, int64(0), calculateCost(0, 0, 1000, 1500, 1000000))
}

func TestMissingPriceScriptResult(t *testing.T) {
	require.True(t, isMissingPriceResult(test.CreateRedisRespArray([]interface{}{0, "missing_price"})))
	require.False(t, isMissingPriceResult(test.CreateRedisRespArray([]interface{}{5, 2, 3})))
	require.False(t, isMissingPriceResult(test.CreateRedisRespError(errors.New("redis failed").Error())))
}

func TestKeyBuilders(t *testing.T) {
	config := QuotaConfig{
		BalanceKeyTemplate: "billing:balance:{tenant}:{quota_scope}:{consumer}",
		PriceKeyTemplate:   "billing:effective_price:{tenant}:{provider}:{model}:{token_type}",
		QuotaScope:         "team-a",
		Provider:           "openai",
	}

	require.Equal(t, "billing:balance:t1:team-a:c1", config.buildBalanceKey("t1", "c1"))
	require.Equal(t, "billing:effective_price:t1:openai:gpt-4:input", config.buildPriceKey("t1", "gpt-4", "input"))
	require.False(t, isAIPathEnabled("/v1/chat/completions/quota", []string{"/v1/chat/completions"}))
	require.True(t, isAIPathEnabled("/proxy/v1/messages?debug=true", []string{"/v1/messages"}))
	require.False(t, strings.Contains(MonetaryDeductionScript, "redis_key_prefix"))
}
