package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

const (
	thinResponseCaptureStreamName      = "cache:events"
	thinResponseCaptureStreamField     = "event"
	thinResponseCaptureSensitiveHeader = "x-mse-cache-sensitive"
)

func thinResponseCaptureConfig(t *testing.T) json.RawMessage {
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
		"redis_stream": map[string]interface{}{
			"enabled":      true,
			"service_name": "redis.static",
			"service_port": 6379,
			"stream":       thinResponseCaptureStreamName,
			"field":        thinResponseCaptureStreamField,
			"timeout":      120,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
		"cacheKeyStrategy":     "lastQuestion",
		"cacheKeyFrom":         "messages.@reverse.0.content",
		"cacheValueFrom":       "choices.0.message.content",
		"cacheStreamValueFrom": "choices.0.delta.content",
	})
	require.NoError(t, err)
	return data
}

func startThinResponseCaptureRequest(t *testing.T, stream bool, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinResponseCaptureConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	require.NoError(t, host.SetRequestId("request-capture-1"))

	headers := [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
	}
	headers = append(headers, extraHeaders...)

	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)

	body, err := json.Marshal(map[string]interface{}{
		"model": "qwen-turbo",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "weather?"},
		},
		"stream": stream,
	})
	require.NoError(t, err)

	action = host.CallOnHttpRequestBody(body)
	require.Equal(t, types.ActionPause, action)
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before upstream capture")

	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	return host
}

func TestThinResponseCapture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming upstream response emits assistant text usage and finish reason", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-response-capture",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "captured non-stream answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "captured non-stream answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "qwen-turbo", event["model"])
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.Equal(t, "consumer", event["cache_scope"])
			require.Equal(t, "policy-v1", event["cache_policy_version"])
			require.EqualValues(t, 200, event["status_code"])
			requireThinResponseCaptureUsage(t, event, 9, 4, 13)
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("streaming upstream response emits concatenated text usage and finish reason", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, true)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callThinResponseStreamChunks(t, host,
				`data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"captured "},"finish_reason":null}]}

data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"stream answer"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "captured stream answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, true, event["is_stream"])
			require.Equal(t, "qwen-turbo", event["model"])
			require.EqualValues(t, 200, event["status_code"])
			requireThinResponseCaptureUsage(t, event, 10, 5, 15)
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("tool-call response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-tool-call",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "do not materialize tool-call content",
						"tool_calls": [{"id":"call-1","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]
					},
					"finish_reason": "tool_calls"
				}],
				"usage": {"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200, "contains_tool_calls")
		})

		t.Run("streaming tool-call response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, true)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callThinResponseStreamChunks(t, host,
				`data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"do not materialize streaming tool content"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}

data: [DONE]

`)

			requireThinResponseCaptureGate(t, host, 200, "contains_tool_calls")
		})

		t.Run("no-store response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
				{"cache-control", "no-store"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-no-store",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "do not materialize no-store content"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 7, "completion_tokens": 4, "total_tokens": 11}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200, "no_store")
		})

		t.Run("sensitive marker does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false, [2]string{thinResponseCaptureSensitiveHeader, "true"})
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-sensitive",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "do not materialize sensitive content"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 6, "completion_tokens": 4, "total_tokens": 10}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200, "sensitive")
		})

		t.Run("failed upstream status does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-failed",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "do not materialize failed response content"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 500)
		})
	})
}

func callThinResponseStreamChunks(t *testing.T, host test.TestHost, chunks ...string) {
	t.Helper()
	for i, chunk := range chunks {
		action := host.CallOnHttpStreamingResponseBody([]byte(chunk), i == len(chunks)-1)
		require.Equal(t, types.ActionContinue, action)
	}
}

func requireThinResponseCaptureEvent(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	event, ok := thinResponseCaptureEvent(t, host)
	if !ok {
		require.Failf(t, "missing thin CacheEvent XADD", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
	}
	requireThinResponseMaterializedLookup(t, host)
	return event
}

func requireThinResponseCaptureGate(t *testing.T, host test.TestHost, statusCode int, markers ...string) {
	t.Helper()
	requireThinResponseNoLegacyCacheSet(t, host)
	event := requireThinResponseCaptureEvent(t, host)
	if _, exists := event["assistant_content"]; exists {
		require.Failf(t, "gated response emitted cacheable assistant content", "event=%s", thinResponseCaptureEventSnippet(t, event))
	}
	require.EqualValuesf(t, statusCode, event["status_code"], "event should record gated response status; event=%s", thinResponseCaptureEventSnippet(t, event))
	for _, marker := range markers {
		require.Equalf(t, true, event[marker], "event should mark %s gate; event=%s", marker, thinResponseCaptureEventSnippet(t, event))
	}
}

func requireThinResponseNoLegacyCacheSet(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 3 {
			continue
		}
		if strings.EqualFold(cmd[0], "set") && strings.HasPrefix(cmd[1], "higress-ai-cache:") {
			require.Failf(t, "thin response capture emitted legacy cache SET", "command=%s", thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
		}
	}
}

func requireThinResponseMaterializedLookup(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 {
			continue
		}
		if strings.EqualFold(cmd[0], "get") && strings.HasPrefix(cmd[1], "cache:materialized:") {
			return
		}
	}
	require.Failf(t, "missing thin materialized Redis lookup", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
}

func requireThinResponseCaptureUsage(t *testing.T, event map[string]interface{}, promptTokens, completionTokens, totalTokens int) {
	t.Helper()
	usage, ok := event["usage"].(map[string]interface{})
	require.Truef(t, ok, "event usage must be an object; event=%s", thinResponseCaptureEventSnippet(t, event))
	require.EqualValues(t, promptTokens, usage["prompt_tokens"])
	require.EqualValues(t, completionTokens, usage["completion_tokens"])
	require.EqualValues(t, totalTokens, usage["total_tokens"])
}

func thinResponseCaptureEvent(t *testing.T, host test.TestHost) (map[string]interface{}, bool) {
	t.Helper()
	var events []map[string]interface{}
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 5 {
			continue
		}
		if !strings.EqualFold(cmd[0], "xadd") || cmd[1] != thinResponseCaptureStreamName {
			continue
		}
		fieldStart := thinResponseCaptureXADDFieldStart(t, cmd)
		require.Equalf(t, 0, (len(cmd)-fieldStart)%2, "cache event XADD has incomplete field/value pair; command=%s", thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
		eventFound := false
		for i := fieldStart; i+1 < len(cmd); i += 2 {
			if cmd[i] != thinResponseCaptureStreamField {
				continue
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(cmd[i+1]), &event); err != nil {
				require.Failf(t, "CacheEvent field is not valid JSON", "err=%v event=%s", err, thinResponseCaptureSnippet(cmd[i+1], 360))
			}
			events = append(events, event)
			eventFound = true
			break
		}
		if !eventFound {
			require.Failf(t, "cache event XADD missing event field", "command=%s", thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
		}
	}
	if len(events) == 0 {
		return nil, false
	}
	require.Lenf(t, events, 1, "expected exactly one CacheEvent XADD; redis calls=%s", thinResponseCaptureCallSummary(t, host))
	return events[0], true
}

func thinResponseCaptureXADDFieldStart(t *testing.T, cmd []string) int {
	t.Helper()
	for i := 2; i < len(cmd); {
		switch strings.ToLower(cmd[i]) {
		case "nomkstream":
			i++
		case "maxlen", "minid":
			i++
			if i < len(cmd) && (cmd[i] == "=" || cmd[i] == "~") {
				i++
			}
			require.Lessf(t, i, len(cmd), "cache event XADD missing trim threshold; command=%s", thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
			i++
			if i < len(cmd) && strings.EqualFold(cmd[i], "limit") {
				i++
				require.Lessf(t, i, len(cmd), "cache event XADD missing LIMIT count; command=%s", thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
				i++
			}
		default:
			streamID := cmd[i]
			require.Truef(t, streamID == "*" || strings.Contains(streamID, "-"), "cache event XADD has invalid stream id %q; command=%s", streamID, thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
			return i + 1
		}
	}
	require.Failf(t, "cache event XADD missing stream id", "command=%s", thinResponseCaptureSnippet(strings.Join(cmd, " "), 360))
	return len(cmd)
}

func thinResponseCaptureCommand(t *testing.T, query []byte) ([]string, bool) {
	t.Helper()
	value, _, err := resp.NewReader(bytes.NewReader(query)).ReadValue()
	if err != nil {
		return nil, false
	}
	array := value.Array()
	if len(array) == 0 {
		return nil, false
	}
	cmd := make([]string, 0, len(array))
	for _, item := range array {
		cmd = append(cmd, item.String())
	}
	return cmd, true
}

func thinResponseCaptureCallSummary(t *testing.T, host test.TestHost) string {
	t.Helper()
	var parts []string
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if ok {
			parts = append(parts, fmt.Sprintf("%s %s", call.Upstream, strings.Join(cmd, " ")))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", call.Upstream, thinResponseCaptureSnippet(string(call.Query), 160)))
	}
	return thinResponseCaptureSnippet(strings.Join(parts, " | "), 600)
}

func thinResponseCaptureEventSnippet(t *testing.T, event map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(event)
	require.NoError(t, err)
	return thinResponseCaptureSnippet(string(data), 600)
}

func thinResponseCaptureSnippet(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
