package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	thinConsoleService  = "console.static"
	thinConsoleUpstream = "outbound|8080||console.static"
	thinConsolePath     = "/internal/cache/lookup"
)

func thinConsoleLookupConfig(t *testing.T, routeConsoleEnabled bool) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis.static",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": thinConsoleService,
			"service_port": 8080,
			"path":         thinConsolePath,
			"timeout":      50,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": routeConsoleEnabled,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	return data
}

func startThinConsoleLookupRequest(t *testing.T, routeConsoleEnabled bool) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinConsoleLookupConfig(t, routeConsoleEnabled))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
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
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before Console lookup is considered")
	return host
}

func requireThinConsoleLookupCall(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	callouts := host.GetHttpCalloutAttributes()
	require.Len(t, callouts, 1, "Redis miss with Console lookup enabled should issue exactly one Console callout")
	callout := callouts[0]
	require.Equal(t, thinConsoleUpstream, callout.Upstream)
	method, ok := test.GetHeaderValue(callout.Headers, ":method")
	require.True(t, ok, "Console lookup call should set :method")
	require.Equal(t, "POST", method)
	path, ok := test.GetHeaderValue(callout.Headers, ":path")
	require.True(t, ok, "Console lookup call should set :path")
	require.Equal(t, thinConsolePath, path)
	require.True(t, test.HasHeaderWithValue(callout.Headers, "content-type", "application/json"), "Console lookup call should send a JSON body")

	var facts map[string]interface{}
	if err := json.Unmarshal(callout.Body, &facts); err != nil {
		require.NoErrorf(t, err, "Console lookup call body must be JSON; body=%s", boundedThinConsoleBody(callout.Body))
	}
	require.Equal(t, "tenant-a", facts["tenant"])
	require.Equal(t, "consumer-a", facts["consumer"])
	require.Equal(t, "session-a", facts["session"])
	require.Equal(t, "test-route-default", facts["route"])
	require.Equal(t, "qwen-turbo", facts["model"])
	require.Equal(t, "consumer", facts["cache_scope"])
	require.Equal(t, "policy-v1", facts["cache_policy_version"])
	requestDigest, ok := facts["request_digest"].(string)
	require.True(t, ok, "Console lookup facts should include string request_digest")
	require.NotEmpty(t, requestDigest)
	return facts
}

func requireThinConsoleFailOpen(t *testing.T, host test.TestHost) {
	t.Helper()
	if localResponse := host.GetLocalResponse(); localResponse != nil {
		require.Failf(
			t,
			"Console lookup should fail open to upstream",
			"got local replay status=%d detail=%s body=%s",
			localResponse.StatusCode,
			localResponse.StatusCodeDetail,
			boundedThinConsoleBody(localResponse.Data),
		)
	}
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
}

func boundedThinConsoleBody(body []byte) string {
	text := string(body)
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func TestThinConsoleLookup(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("disabled lookup fails open after Redis miss without HTTP callout", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, false)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			require.Empty(t, host.GetHttpCalloutAttributes(), "route-disabled Console lookup should not issue an HTTP callout")
			requireThinConsoleFailOpen(t, host)
		})

		t.Run("enabled lookup sends facts after Redis miss", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			requireThinConsoleLookupCall(t, host)
		})

		t.Run("hit replays validated OpenAI response", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(validThinReplayRecord(t, nil)))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid Console replay record should be replayed")
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

		t.Run("invalid hit record fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(validThinReplayRecord(t, map[string]interface{}{"schema_version": "ai-cache.materialized.v0"})))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("miss response fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"hit":false}`))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("timeout no-status callback fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall(nil, nil)

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("failure response fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "503"},
				{"content-type", "application/json"},
			}, []byte(`{"error":"console unavailable"}`))

			requireThinConsoleFailOpen(t, host)
		})
	})
}
