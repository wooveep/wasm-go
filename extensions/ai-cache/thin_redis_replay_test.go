package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func thinRedisReplayConfig(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":           "redis",
			"serviceName":    "redis.static",
			"servicePort":    6379,
			"cacheKeyPrefix": "higress-ai-cache:",
		},
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis.static",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	return data
}

func validThinReplayRecord(t *testing.T, overrides map[string]interface{}) string {
	t.Helper()
	now := time.Now().Unix()
	record := map[string]interface{}{
		"schema_version":       "ai-cache.materialized.v1",
		"tenant":               "tenant-a",
		"consumer":             "consumer-a",
		"route":                "test-route-default",
		"model":                "qwen-turbo",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
		"request_digest":       "digest-placeholder",
		"soft_expires_at":      now + 60,
		"hard_expires_at":      now + 3600,
		"response": map[string]interface{}{
			"id":     "chatcmpl-cache-hit",
			"object": "chat.completion",
			"model":  "qwen-turbo",
			"choices": []interface{}{
				map[string]interface{}{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "cached answer",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     7,
			"completion_tokens": 3,
			"total_tokens":      10,
		},
		"finish_reason": "stop",
	}
	for key, value := range overrides {
		record[key] = value
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	return string(body)
}

func startThinRedisReplayRequest(t *testing.T) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinRedisReplayConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
	})
	require.Equal(t, types.HeaderStopIteration, action)
	action = host.CallOnHttpRequestBody([]byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "weather?"}
		],
		"stream": false
	}`))
	require.Equal(t, types.ActionPause, action)
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before replay validation")
	return host
}

func TestThinRedisReplayValidation(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("valid materialized record replays OpenAI response", func(t *testing.T) {
			host := startThinRedisReplayRequest(t)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, nil)))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.JSONEq(t, `{
				"id": "chatcmpl-cache-hit",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "cached answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10}
			}`, string(localResponse.Data))
		})

		tests := []struct {
			name      string
			redisResp []byte
		}{
			{
				name:      "miss",
				redisResp: test.CreateRedisRespNull(),
			},
			{
				name:      "invalid JSON",
				redisResp: test.CreateRedisRespString(`{not-json`),
			},
			{
				name:      "unsupported schema version",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"schema_version": "ai-cache.materialized.v0"})),
			},
			{
				name:      "expired hard expiration",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"hard_expires_at": time.Now().Unix() - 1})),
			},
			{
				name:      "scope mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"consumer": "consumer-b"})),
			},
			{
				name:      "route mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"route": "other-route"})),
			},
			{
				name:      "model mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"model": "other-model"})),
			},
			{
				name:      "policy version mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"cache_policy_version": "policy-v2"})),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name+" fails open", func(t *testing.T) {
				host := startThinRedisReplayRequest(t)
				defer host.Reset()

				host.CallOnRedisCall(0, tt.redisResp)

				if localResponse := host.GetLocalResponse(); localResponse != nil {
					body := string(localResponse.Data)
					if len(body) > 240 {
						body = body[:240] + "..."
					}
					require.Failf(t, "invalid or missing materialized record should fail open to upstream", "got local replay status=%d detail=%s body=%s", localResponse.StatusCode, localResponse.StatusCodeDetail, body)
				}
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			})
		}
	})
}
