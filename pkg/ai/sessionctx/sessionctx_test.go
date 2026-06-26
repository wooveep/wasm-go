package sessionctx_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/stretchr/testify/require"
)

func TestRequestFactsExtraction(t *testing.T) {
	t.Run("custom identity and session headers override defaults", func(t *testing.T) {
		headers := [][2]string{
			{"X-Tenant-ID", "tenant-a"},
			{"x-consumer-id", "consumer-a"},
			{"x-session-id", "session-custom"},
			{"x-openclaw-session-key", "session-default"},
			{"x-request-id", "header-request-id"},
		}
		facts := sessionctx.ExtractRequestFacts(headers, sessionctx.RequestFactOptions{
			TenantHeader:    "x-tenant-id",
			ConsumerHeader:  "x-consumer-id",
			SessionHeader:   "x-session-id",
			RequestIDHeader: "x-request-id",
		}, "property-request-id")

		require.Equal(t, "tenant-a", facts.Tenant)
		require.Equal(t, "consumer-a", facts.Consumer)
		require.Equal(t, "session-custom", facts.SessionID)
		require.Equal(t, "header-request-id", facts.RequestID)
		require.Equal(t, "tenant-a", sessionctx.HeaderValue(headers, "x-tenant-id"))
	})

	t.Run("default headers and request property fallback are stable", func(t *testing.T) {
		facts := sessionctx.ExtractRequestFacts([][2]string{
			{"X-Mse-Tenant", "tenant-default"},
			{"X-Mse-Consumer", "consumer-default"},
			{"x-agent-session", "session-agent"},
			{"x-openclaw-session-key", "session-openclaw"},
		}, sessionctx.RequestFactOptions{}, "property-request-id")

		require.Equal(t, "tenant-default", facts.Tenant)
		require.Equal(t, "consumer-default", facts.Consumer)
		require.Equal(t, "session-openclaw", facts.SessionID)
		require.Equal(t, "property-request-id", facts.RequestID)
	})
}

func TestRequestGateHelpers(t *testing.T) {
	require.True(t, sessionctx.IsJSONContentType("application/json"))
	require.True(t, sessionctx.IsJSONContentType("application/json; charset=utf-8"))
	require.True(t, sessionctx.IsJSONContentType("application/problem+json"))
	require.False(t, sessionctx.IsJSONContentType("text/plain"))

	require.True(t, sessionctx.PathMatchesSuffixes("/v1/chat/completions", []string{"/chat/completions"}))
	require.True(t, sessionctx.PathMatchesSuffixes("/v1/chat/completions?api-version=2024-01-01", []string{"/chat/completions"}))
	require.True(t, sessionctx.PathMatchesSuffixes("/v1/chat/completions", nil))
	require.False(t, sessionctx.PathMatchesSuffixes("/v1/embeddings", []string{"/chat/completions"}))
	require.False(t, sessionctx.PathMatchesSuffixes("/v1/embeddings", []string{""}))
}

func TestOpenAIRequestParsingIntentAndDigest(t *testing.T) {
	body := []byte(`{
		"model": "qwen-turbo",
		"stream": true,
		"tools": [{"type":"function","function":{"name":"lookup_weather","parameters":{"type":"object"}}}],
		"tool_choice": {"function":{"name":"lookup_weather"},"type":"function"},
		"response_format": {"type":"json_object"},
		"messages": [
			{"role": "system", "content": "answer tersely"},
			{"role": "user", "content": "older question"},
			{"role": "assistant", "content": "older answer"},
			{"role": "user", "content": [
				{"type": "text", "text": "latest weather intent"},
				{"type": "image_url", "image_url": {"url": "https://example.invalid/image.png"}}
			]}
		]
	}`)

	request, err := sessionctx.ParseOpenAIChatRequest(body)
	require.NoError(t, err)
	require.Equal(t, "qwen-turbo", request.Model)
	require.True(t, request.Stream)
	require.Len(t, request.Messages, 4)
	require.NotEmpty(t, request.Tools)
	require.NotEmpty(t, request.ToolChoice)
	require.NotEmpty(t, request.ResponseFormat)
	require.Equal(t, "latest weather intent", sessionctx.CurrentUserIntent(request.Messages))

	digestA, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:          request.Model,
		Messages:       request.Messages,
		Tools:          request.Tools,
		ToolChoice:     request.ToolChoice,
		ResponseFormat: request.ResponseFormat,
	})
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), digestA)

	sameRequest, err := sessionctx.ParseOpenAIChatRequest([]byte(`{"stream":true,"response_format":{"type":"json_object"},"tool_choice":{"type":"function","function":{"name":"lookup_weather"}},"tools":[{"function":{"parameters":{"type":"object"},"name":"lookup_weather"},"type":"function"}],"messages":[{"content":"answer tersely","role":"system"},{"content":"older question","role":"user"},{"content":"older answer","role":"assistant"},{"content":[{"text":"latest weather intent","type":"text"},{"image_url":{"url":"https://example.invalid/image.png"},"type":"image_url"}],"role":"user"}],"model":"qwen-turbo"}`))
	require.NoError(t, err)
	digestB, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:          sameRequest.Model,
		Messages:       sameRequest.Messages,
		Tools:          sameRequest.Tools,
		ToolChoice:     sameRequest.ToolChoice,
		ResponseFormat: sameRequest.ResponseFormat,
	})
	require.NoError(t, err)
	require.Equal(t, digestA, digestB)

	changedModel, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:    "qwen-plus",
		Messages: sameRequest.Messages,
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedModel)

	changedToolChoice, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:          request.Model,
		Messages:       request.Messages,
		Tools:          request.Tools,
		ToolChoice:     jsonRaw(`{"type":"function","function":{"name":"lookup_other"}}`),
		ResponseFormat: request.ResponseFormat,
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedToolChoice)
}

func TestOpenAIResponseParsing(t *testing.T) {
	response, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"id": "chatcmpl-1",
		"model": "qwen-turbo",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "assistant answer"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
	}`))
	require.NoError(t, err)
	require.Equal(t, "assistant answer", response.AssistantContent)
	require.Equal(t, "stop", response.FinishReason)
	require.Equal(t, sessionctx.Usage{
		PromptTokens:     9,
		CompletionTokens: 4,
		TotalTokens:      13,
	}, response.Usage)
	require.False(t, response.ContainsToolCalls)

	toolResponse, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"choices": [{
			"message": {
				"role": "assistant",
				"tool_calls": [{"id": "call-1", "type": "function", "function": {"name": "lookup", "arguments": "{}"}}]
			},
			"finish_reason": "tool_calls"
		}]
	}`))
	require.NoError(t, err)
	require.True(t, toolResponse.ContainsToolCalls)
	require.Equal(t, "tool_calls", toolResponse.FinishReason)

	functionResponse, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"choices": [{
			"message": {
				"role": "assistant",
				"function_call": {"name": "legacy_lookup", "arguments": "{}"}
			},
			"finish_reason": "function_call"
		}]
	}`))
	require.NoError(t, err)
	require.True(t, functionResponse.ContainsToolCalls)
}

func TestStreamCapture(t *testing.T) {
	capture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})

	require.NoError(t, capture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n")))
	require.NoError(t, capture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"},\"finish_reason\":\"stop\"}]}\n\n")))
	require.NoError(t, capture.AppendSSE([]byte("data: [DONE]\n\n")))

	require.Equal(t, "hello world", capture.AssistantContent())
	require.Equal(t, "stop", capture.FinishReason())
	require.False(t, capture.ContainsToolCalls())

	toolCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, toolCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]}}]}\n\n")))
	require.True(t, toolCapture.ContainsToolCalls())

	functionCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, functionCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"function_call\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}}]}\n\n")))
	require.True(t, functionCapture.ContainsToolCalls())

	splitCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, splitCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"split")))
	require.NoError(t, splitCapture.AppendSSE([]byte(" chunk\"},\"finish_reason\":\"stop\"}]}\n\n")))
	require.Equal(t, "split chunk", splitCapture.AssistantContent())
	require.Equal(t, "stop", splitCapture.FinishReason())
}

func TestEventEnvelopeAndSafeLogRedaction(t *testing.T) {
	input := sessionctx.EventEnvelopeInput{
		EventKind:     "cache",
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RequestID:     "request-a",
		RequestDigest: "digest-a",
		StartedAtMS:   1000,
		EndedAtMS:     1500,
	}
	event := sessionctx.NewEventEnvelope(input)
	repeated := sessionctx.NewEventEnvelope(input)
	delayed := sessionctx.NewEventEnvelope(sessionctx.EventEnvelopeInput{
		EventKind:     "cache",
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RequestID:     "request-a",
		RequestDigest: "digest-a",
		StartedAtMS:   2000,
		EndedAtMS:     2500,
	})

	require.NotEmpty(t, event.EventID)
	require.Equal(t, "cache", event.EventKind)
	require.Equal(t, "tenant-a", event.Tenant)
	require.Equal(t, "consumer-a", event.Consumer)
	require.Equal(t, "request-a", event.RequestID)
	require.Equal(t, "digest-a", event.RequestDigest)
	require.EqualValues(t, 1000, event.StartedAtMS)
	require.EqualValues(t, 1500, event.EndedAtMS)
	require.Equal(t, event.IdempotencyKey, repeated.IdempotencyKey)
	require.Equal(t, event.IdempotencyKey, delayed.IdempotencyKey)

	redacted := sessionctx.RedactForLog(`authorization=Bearer credential-one x-api-key: credential-two "x-internal-bearer":"credential-three" x-redis-password=credential-four x-provider-api-key=credential-five route=chat`)
	for _, item := range []struct {
		label string
		value string
	}{
		{label: "authorization value", value: "credential-one"},
		{label: "api key value", value: "credential-two"},
		{label: "internal bearer value", value: "credential-three"},
		{label: "redis credential value", value: "credential-four"},
		{label: "provider credential value", value: "credential-five"},
	} {
		requireStringExcludes(t, redacted, item.label, item.value)
	}
	require.Contains(t, redacted, "route=chat")
	require.Contains(t, strings.ToLower(redacted), "redacted")
}

func requireStringExcludes(t *testing.T, text, label, value string) {
	t.Helper()
	require.Falsef(t, strings.Contains(text, value), "redacted output leaked %s", label)
}

func jsonRaw(value string) json.RawMessage {
	return json.RawMessage(value)
}
