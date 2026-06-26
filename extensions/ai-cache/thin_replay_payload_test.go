package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func startThinReplayPayloadRequest(t *testing.T, stream bool) test.TestHost {
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
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before replay")
	return host
}

func TestThinReplayPayload(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming replay preserves embedded OpenAI response fields", func(t *testing.T) {
			host := startThinReplayPayloadRequest(t, false)
			defer host.Reset()

			response := map[string]interface{}{
				"id":                 "chatcmpl-preserved-42",
				"object":             "chat.completion",
				"created":            float64(1720000123),
				"model":              "qwen-turbo",
				"system_fingerprint": "fp-cache-preserved",
				"choices": []interface{}{
					map[string]interface{}{
						"index": float64(0),
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "cached weather answer with preserved fields",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     float64(11),
					"completion_tokens": float64(7),
					"total_tokens":      float64(18),
				},
			}

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{
				"response": response,
				"usage":    response["usage"],
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))
			thinReplayPayloadRequireJSONEqual(t, thinReplayPayloadMustJSON(t, response), localResponse.Data)
		})

		t.Run("streaming replay emits stored chunks and done marker", func(t *testing.T) {
			host := startThinReplayPayloadRequest(t, true)
			defer host.Reset()

			chunks := []interface{}{
				map[string]interface{}{
					"id":                 "chatcmpl-stream-hit",
					"object":             "chat.completion.chunk",
					"created":            float64(1720000456),
					"model":              "qwen-turbo",
					"system_fingerprint": "fp-cache-stream",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         float64(0),
							"delta":         map[string]interface{}{"role": "assistant"},
							"finish_reason": nil,
						},
					},
				},
				map[string]interface{}{
					"id":                 "chatcmpl-stream-hit",
					"object":             "chat.completion.chunk",
					"created":            float64(1720000456),
					"model":              "qwen-turbo",
					"system_fingerprint": "fp-cache-stream",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         float64(0),
							"delta":         map[string]interface{}{"content": "cached stream answer"},
							"finish_reason": nil,
						},
					},
				},
				map[string]interface{}{
					"id":                 "chatcmpl-stream-hit",
					"object":             "chat.completion.chunk",
					"created":            float64(1720000456),
					"model":              "qwen-turbo",
					"system_fingerprint": "fp-cache-stream",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         float64(0),
							"delta":         map[string]interface{}{},
							"finish_reason": "stop",
						},
					},
				},
			}

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{
				"stream_replayable": true,
				"stream_chunks":     chunks,
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "stream-replayable materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "text/event-stream; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))

			frames := thinReplayPayloadSSEDataFrames(t, localResponse.Data)
			if len(frames) != len(chunks)+1 {
				require.Failf(t, "unexpected SSE frame count", "expected=%d actual=%d body=%s", len(chunks)+1, len(frames), thinReplayPayloadSnippet(localResponse.Data, 360))
			}
			for i, chunk := range chunks {
				thinReplayPayloadRequireJSONEqual(t, thinReplayPayloadMustJSON(t, chunk), []byte(frames[i]))
			}
			require.Equal(t, "[DONE]", frames[len(frames)-1])
		})
	})
}

func thinReplayPayloadHeader(headers [][2]string, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			return header[1]
		}
	}
	return ""
}

func thinReplayPayloadSSEDataFrames(t *testing.T, body []byte) []string {
	t.Helper()
	lines := strings.Split(string(body), "\n")
	frames := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "data:"):
			frames = append(frames, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		default:
			require.Failf(t, "unexpected SSE line", "line=%q body=%s", line, thinReplayPayloadSnippet(body, 360))
		}
	}
	return frames
}

func thinReplayPayloadRequireJSONEqual(t *testing.T, expected string, actual []byte) {
	t.Helper()
	var expectedValue interface{}
	require.NoError(t, json.Unmarshal([]byte(expected), &expectedValue))

	var actualValue interface{}
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		require.Failf(t, "invalid JSON body", "err=%v body=%s", err, thinReplayPayloadSnippet(actual, 360))
	}
	if !reflect.DeepEqual(expectedValue, actualValue) {
		require.Failf(t, "JSON payload mismatch", "expected=%s actual=%s", expected, thinReplayPayloadSnippet(actual, 360))
	}
}

func thinReplayPayloadMustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func thinReplayPayloadSnippet(data []byte, limit int) string {
	body := string(data)
	if len(body) <= limit {
		return body
	}
	return body[:limit] + "..."
}
