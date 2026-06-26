package sessionctx_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/stretchr/testify/require"
)

func TestRequestFactsExtraction(t *testing.T) {
	t.Run("custom identity and session headers override defaults", func(t *testing.T) {
		facts := sessionctx.ExtractRequestFacts([][2]string{
			{"X-Tenant-ID", "tenant-a"},
			{"x-consumer-id", "consumer-a"},
			{"x-session-id", "session-custom"},
			{"x-openclaw-session-key", "session-default"},
			{"x-request-id", "header-request-id"},
		}, sessionctx.RequestFactOptions{
			TenantHeader:    "x-tenant-id",
			ConsumerHeader:  "x-consumer-id",
			SessionHeader:   "x-session-id",
			RequestIDHeader: "x-request-id",
		}, "property-request-id")

		require.Equal(t, "tenant-a", facts.Tenant)
		require.Equal(t, "consumer-a", facts.Consumer)
		require.Equal(t, "session-custom", facts.SessionID)
		require.Equal(t, "header-request-id", facts.RequestID)
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

func TestOpenAIRequestParsingIntentAndDigest(t *testing.T) {
	body := []byte(`{
		"model": "qwen-turbo",
		"stream": true,
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
	require.Equal(t, "latest weather intent", sessionctx.CurrentUserIntent(request.Messages))

	digestA, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:    request.Model,
		Messages: request.Messages,
	})
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), digestA)

	sameRequest, err := sessionctx.ParseOpenAIChatRequest([]byte(`{"stream":true,"messages":[{"content":"answer tersely","role":"system"},{"content":"older question","role":"user"},{"content":"older answer","role":"assistant"},{"content":[{"text":"latest weather intent","type":"text"},{"image_url":{"url":"https://example.invalid/image.png"},"type":"image_url"}],"role":"user"}],"model":"qwen-turbo"}`))
	require.NoError(t, err)
	digestB, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:    sameRequest.Model,
		Messages: sameRequest.Messages,
	})
	require.NoError(t, err)
	require.Equal(t, digestA, digestB)

	changedModel, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:    "qwen-plus",
		Messages: sameRequest.Messages,
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedModel)
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

	require.NotEmpty(t, event.EventID)
	require.Equal(t, "cache", event.EventKind)
	require.Equal(t, "tenant-a", event.Tenant)
	require.Equal(t, "consumer-a", event.Consumer)
	require.Equal(t, "request-a", event.RequestID)
	require.Equal(t, "digest-a", event.RequestDigest)
	require.EqualValues(t, 1000, event.StartedAtMS)
	require.EqualValues(t, 1500, event.EndedAtMS)
	require.Equal(t, event.IdempotencyKey, repeated.IdempotencyKey)

	redacted := sessionctx.RedactForLog("authorization=credential-one x-api-key=credential-two x-internal-bearer=credential-three x-redis-password=credential-four x-provider-api-key=credential-five route=chat")
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
