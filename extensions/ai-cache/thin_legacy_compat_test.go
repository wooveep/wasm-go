package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestThinLegacyCompatibility(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("request lookup uses materialized Redis key and not legacy cache prefix", func(t *testing.T) {
			host := startThinLegacyCompatibilityRequest(t)
			defer host.Reset()

			requireThinResponseMaterializedLookup(t, host)
			requireThinLegacyCompatibilityNoLegacyCacheCalls(t, host)
		})

		t.Run("legacy text-only Redis hit fails open instead of replaying response template", func(t *testing.T) {
			host := startThinLegacyCompatibilityRequest(t)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespString("legacy cached text answer"))

			requireThinLegacyCompatibilityFailOpen(t, host)
			requireThinLegacyCompatibilityNoLegacyCacheCalls(t, host)
		})

		t.Run("Redis miss does not invoke legacy embedding or vector providers", func(t *testing.T) {
			host := startThinLegacyCompatibilityRequest(t)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			requireThinLegacyCompatibilityNoHTTPCallouts(t, host, "thin production default path must not call legacy embedding/vector services")
			requireThinLegacyCompatibilityFailOpen(t, host)
			requireThinLegacyCompatibilityNoLegacyCacheCalls(t, host)
		})

		t.Run("upstream response capture does not invoke legacy embedding or vector upload", func(t *testing.T) {
			host := startThinLegacyCompatibilityRequest(t)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			driveThinLegacyCompatibilitySemanticMiss(t, host)
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-legacy-compat",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "upstream answer should not be vectorized"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 7, "completion_tokens": 5, "total_tokens": 12}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinLegacyCompatibilityNoVectorUploadCallout(t, host)
			requireThinLegacyCompatibilityNoLegacyCacheCalls(t, host)
		})
	})
}

func thinLegacyCompatibilityConfig(t *testing.T) json.RawMessage {
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
		"embedding": map[string]interface{}{
			"type":        "dashscope",
			"apiKey":      "thin-legacy-placeholder",
			"serviceName": "dashscope.static",
			"servicePort": 8080,
		},
		"vector": map[string]interface{}{
			"type":              "dashvector",
			"serviceName":       "dashvector-service",
			"serviceHost":       "dashvector.example.com",
			"servicePort":       8081,
			"apiKey":            "thin-legacy-placeholder",
			"collectionID":      "test-collection",
			"threshold":         0.8,
			"thresholdRelation": "gt",
		},
		"enableSemanticCache":    true,
		"tenant_header":          "x-mse-tenant",
		"consumer_header":        "x-mse-consumer",
		"session_header":         "x-mse-session",
		"cache_scope":            "consumer",
		"cache_policy_version":   "policy-v1",
		"cacheKeyStrategy":       "lastQuestion",
		"cacheKeyFrom":           "messages.@reverse.0.content",
		"cacheValueFrom":         "choices.0.message.content",
		"cacheStreamValueFrom":   "choices.0.delta.content",
		"responseTemplate":       `{"id":"legacy-template","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"legacy-template","object":"chat.completion"}`,
		"streamResponseTemplate": `data:{"id":"legacy-template","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"legacy-template","object":"chat.completion.chunk"}` + "\n\ndata:[DONE]\n\n",
	})
	require.NoError(t, err)
	return data
}

func startThinLegacyCompatibilityRequest(t *testing.T) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinLegacyCompatibilityConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	require.NoError(t, host.SetRequestId("request-legacy-compat-1"))

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
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
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "thin path should issue a materialized Redis lookup before upstream")
	return host
}

func requireThinLegacyCompatibilityFailOpen(t *testing.T, host test.TestHost) {
	t.Helper()
	if localResponse := host.GetLocalResponse(); localResponse != nil {
		require.Failf(
			t,
			"thin production default path should fail open instead of using legacy replay",
			"got local replay status=%d detail=%s body=%s redis calls=%s",
			localResponse.StatusCode,
			localResponse.StatusCodeDetail,
			thinLegacyCompatibilitySnippet(string(localResponse.Data), 240),
			thinResponseCaptureCallSummary(t, host),
		)
	}
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
}

func driveThinLegacyCompatibilitySemanticMiss(t *testing.T, host test.TestHost) {
	t.Helper()
	if !thinLegacyCompatibilityHasHTTPCalloutPath(t, host, "/api/v1/services/embeddings/text-embedding/text-embedding") {
		return
	}
	host.CallOnHttpCall([][2]string{
		{"Content-Type", "application/json"},
		{":status", "200"},
	}, []byte(`{
		"output": {
			"embeddings": [
				{"embedding": [0.1, 0.2, 0.3, 0.4, 0.5]}
			]
		}
	}`))

	if !thinLegacyCompatibilityHasHTTPCalloutPath(t, host, "/v1/collections/test-collection/query") {
		return
	}
	host.CallOnHttpCall([][2]string{
		{"Content-Type", "application/json"},
		{":status", "200"},
	}, []byte(`{
		"code": 200,
		"request_id": "thin-legacy-compat",
		"message": "success",
		"output": []
	}`))
}

func requireThinLegacyCompatibilityNoHTTPCallouts(t *testing.T, host test.TestHost, message string) {
	t.Helper()
	if len(host.GetHttpCalloutAttributes()) == 0 {
		return
	}
	require.Failf(t, message, "http calls=%s", thinLegacyCompatibilityHTTPCallSummary(t, host))
}

func requireThinLegacyCompatibilityNoVectorUploadCallout(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetHttpCalloutAttributes() {
		path, _ := test.GetHeaderValue(call.Headers, ":path")
		if strings.Contains(path, "/v1/collections/test-collection/docs") {
			require.Failf(
				t,
				"thin response capture must not call legacy vector upload service",
				"http calls=%s",
				thinLegacyCompatibilityHTTPCallSummary(t, host),
			)
		}
	}
}

func requireThinLegacyCompatibilityNoLegacyCacheCalls(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 {
			continue
		}
		if strings.EqualFold(cmd[0], "get") || strings.EqualFold(cmd[0], "set") {
			require.Falsef(
				t,
				strings.HasPrefix(cmd[1], "higress-ai-cache:"),
				"thin production default path must not use legacy Redis cache prefix; redis calls=%s",
				thinResponseCaptureCallSummary(t, host),
			)
		}
	}
}

func thinLegacyCompatibilityHasHTTPCalloutPath(t *testing.T, host test.TestHost, expectedPath string) bool {
	t.Helper()
	for _, call := range host.GetHttpCalloutAttributes() {
		path, _ := test.GetHeaderValue(call.Headers, ":path")
		if path == expectedPath {
			return true
		}
	}
	return false
}

func thinLegacyCompatibilityHTTPCallSummary(t *testing.T, host test.TestHost) string {
	t.Helper()
	var parts []string
	for _, call := range host.GetHttpCalloutAttributes() {
		path, _ := test.GetHeaderValue(call.Headers, ":path")
		method, _ := test.GetHeaderValue(call.Headers, ":method")
		parts = append(parts, fmt.Sprintf("%s %s %s body=<%d>", call.Upstream, method, path, len(call.Body)))
	}
	return thinLegacyCompatibilitySnippet(strings.Join(parts, " | "), 600)
}

func thinLegacyCompatibilitySnippet(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
