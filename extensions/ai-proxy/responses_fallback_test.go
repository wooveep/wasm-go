package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/test"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var responsesNativeOpenAIConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":      "openai",
			"apiTokens": []string{"sk-openai-responses-native"},
		},
	})
	return data
}()

var responsesFallbackOpenRouterConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":      "openrouter",
			"apiTokens": []string{"sk-openrouter-responses-fallback"},
		},
	})
	return data
}()

var responsesUnsupportedKlingConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":           "kling",
			"klingAccessKey": "kling-ak-test",
			"klingSecretKey": "kling-sk-test",
		},
	})
	return data
}()

func TestResponsesResponseConversionMarker(t *testing.T) {
	ctx := test.NewMockHttpContext()
	require.False(t, needsResponsesResponseConversion(ctx))

	ctx.SetContext("needResponsesResponseConversion", true)
	require.True(t, needsResponsesResponseConversion(ctx))

	ctx.SetContext("needClaudeResponseConversion", true)
	require.True(t, needsResponsesResponseConversion(ctx))
	require.True(t, NeedsClaudeResponseConversionForTest(ctx))
}

func TestResponsesNativeRoutingKeepsResponsesPath(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesNativeOpenAIConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/v1/responses", pathValue)
	})
}

func TestResponsesFallbackRoutingUsesChatCompletions(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/api/v1/chat/completions", pathValue)
	})
}

func TestResponsesFallbackRequestBodyConvertsToChatCompletions(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(`{"model":"openai/gpt-4o-mini","input":"hello","max_output_tokens":32}`))
		require.Equal(t, types.ActionContinue, action)

		body := host.GetRequestBody()
		require.Equal(t, "user", gjson.GetBytes(body, "messages.0.role").String())
		require.Equal(t, "hello", gjson.GetBytes(body, "messages.0.content").String())
		require.Equal(t, int64(32), gjson.GetBytes(body, "max_completion_tokens").Int())
		require.False(t, gjson.GetBytes(body, "input").Exists(), string(body))
	})
}

func TestResponsesUnsupportedProviderDoesNotFallbackToUnrelatedAPI(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesUnsupportedKlingConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/v1/responses", pathValue)

		_ = host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","input":"hello"}`))
		allLogs := strings.Join(append(append([]string{}, host.GetErrorLogs()...), host.GetWarnLogs()...), "\n")
		require.Contains(t, allLogs, "unsupported API name")
	})
}

func TestResponsesFallbackDoesNotRegressMessagesOrChatCompletions(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/messages"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)
		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/api/v1/chat/completions", pathValue)
	})

	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/chat/completions"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)
		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/api/v1/chat/completions", pathValue)
	})
}
