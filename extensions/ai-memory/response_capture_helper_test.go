package main

import (
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/stretchr/testify/require"
)

func TestMemoryCaptureNonStreamingResponseFacts(t *testing.T) {
	t.Run("captures assistant content status usage finish reason and configured tool calls", func(t *testing.T) {
		capture := memoryCaptureNonStreamingResponse([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "captured helper answer",
					"custom_tool_calls": [{"id":"custom-call-1","name":"lookup"}]
				},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
		}`), 202, "choices.0.message.content", []string{"choices.0.message.custom_tool_calls"})

		require.Equal(t, "captured helper answer", capture.AssistantContent)
		require.Equal(t, "stop", capture.FinishReason)
		require.Equal(t, 202, capture.StatusCode)
		require.False(t, capture.IsStream)
		require.True(t, capture.ContainsToolCalls)
		require.False(t, capture.ParseFailed)
		require.Equal(t, 9, capture.Usage.PromptTokens)
		require.Equal(t, 4, capture.Usage.CompletionTokens)
		require.Equal(t, 13, capture.Usage.TotalTokens)
	})

	t.Run("uses configured response value path for assistant content", func(t *testing.T) {
		capture := memoryCaptureNonStreamingResponse([]byte(`{
			"choices": [{
				"message": {"role": "assistant"},
				"finish_reason": "stop"
			}],
			"output": {"message": {"text": "configured path assistant answer"}},
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
		}`), 200, "output.message.text", nil)

		require.Equal(t, "configured path assistant answer", capture.AssistantContent)
		require.Equal(t, "stop", capture.FinishReason)
		require.False(t, capture.ParseFailed)
		require.Equal(t, 3, capture.Usage.PromptTokens)
		require.Equal(t, 2, capture.Usage.CompletionTokens)
		require.Equal(t, 5, capture.Usage.TotalTokens)
	})

	t.Run("parse failure preserves status and records failure without content", func(t *testing.T) {
		capture := memoryCaptureNonStreamingResponse([]byte(`{not-json`), 502, "choices.0.message.content", nil)

		require.Equal(t, 502, capture.StatusCode)
		require.False(t, capture.IsStream)
		require.True(t, capture.ParseFailed)
		require.Empty(t, capture.AssistantContent)
		require.Empty(t, capture.FinishReason)
		require.False(t, capture.ContainsToolCalls)
		require.Zero(t, capture.Usage)
	})
}

func TestMemoryResponseHookSkipsBodyForGatedAndStreamRequests(t *testing.T) {
	t.Run("gated request disables response body processing", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		markMemoryGate(ctx, "missing-tenant")

		action := onHttpResponseHeaders(ctx, config.PluginConfig{}, memoryCaptureNoopLog{})

		require.Equal(t, types.ActionContinue, action)
		require.True(t, ctx.dontReadResponseBody)
	})

	t.Run("stream request is left for streaming capture task", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		ctx.SetContext(memoryStreamContextKey, true)

		action := onHttpResponseHeaders(ctx, config.PluginConfig{}, memoryCaptureNoopLog{})

		require.Equal(t, types.ActionContinue, action)
		require.True(t, ctx.dontReadResponseBody)
	})
}

func TestMemoryCaptureNonStreamingResponseChunkLimitsBuffer(t *testing.T) {
	t.Run("non-final chunks fail open when combined buffer is too large", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		ctx.SetContext(memoryResponseStatusContextKey, 200)

		memoryCaptureNonStreamingResponseChunkWithLimit(ctx, config.PluginConfig{}, []byte("12345"), false, 8, memoryCaptureNoopLog{})
		memoryCaptureNonStreamingResponseChunkWithLimit(ctx, config.PluginConfig{}, []byte("6789"), false, 8, memoryCaptureNoopLog{})

		capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
		require.True(t, ok)
		require.Equal(t, 200, capture.StatusCode)
		require.True(t, capture.ParseFailed)
		require.Empty(t, ctx.GetByteSliceContext(memoryResponseBodyBufferContextKey, nil))
	})

	t.Run("single final chunk fails open when too large", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		ctx.SetContext(memoryResponseStatusContextKey, 200)

		memoryCaptureNonStreamingResponseChunkWithLimit(ctx, config.PluginConfig{}, []byte(`{"choices":[{"message":{"content":"too large"},"finish_reason":"stop"}]}`), true, 8, memoryCaptureNoopLog{})

		capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
		require.True(t, ok)
		require.Equal(t, 200, capture.StatusCode)
		require.True(t, capture.ParseFailed)
		require.Empty(t, capture.AssistantContent)
	})
}

type memoryCaptureTestContext struct {
	values               map[string]interface{}
	dontReadResponseBody bool
}

func newMemoryCaptureTestContext() *memoryCaptureTestContext {
	return &memoryCaptureTestContext{values: map[string]interface{}{}}
}

func (m *memoryCaptureTestContext) Scheme() string { return "" }
func (m *memoryCaptureTestContext) Host() string   { return "" }
func (m *memoryCaptureTestContext) Path() string   { return "" }
func (m *memoryCaptureTestContext) Method() string { return "" }
func (m *memoryCaptureTestContext) SetContext(key string, value interface{}) {
	m.values[key] = value
}
func (m *memoryCaptureTestContext) GetContext(key string) interface{} { return m.values[key] }
func (m *memoryCaptureTestContext) GetBoolContext(key string, defaultValue bool) bool {
	if value, ok := m.values[key].(bool); ok {
		return value
	}
	return defaultValue
}
func (m *memoryCaptureTestContext) GetStringContext(key, defaultValue string) string {
	if value, ok := m.values[key].(string); ok {
		return value
	}
	return defaultValue
}
func (m *memoryCaptureTestContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	if value, ok := m.values[key].([]byte); ok {
		return value
	}
	return defaultValue
}
func (m *memoryCaptureTestContext) GetUserAttribute(key string) interface{} { return nil }
func (m *memoryCaptureTestContext) SetUserAttribute(key string, value interface{}) {
}
func (m *memoryCaptureTestContext) SetUserAttributeMap(kvmap map[string]interface{}) {}
func (m *memoryCaptureTestContext) GetUserAttributeMap() map[string]interface{} {
	return nil
}
func (m *memoryCaptureTestContext) WriteUserAttributeToLog() error                  { return nil }
func (m *memoryCaptureTestContext) WriteUserAttributeToLogWithKey(key string) error { return nil }
func (m *memoryCaptureTestContext) WriteUserAttributeToTrace() error                { return nil }
func (m *memoryCaptureTestContext) DontReadRequestBody()                            {}
func (m *memoryCaptureTestContext) DontReadResponseBody() {
	m.dontReadResponseBody = true
}
func (m *memoryCaptureTestContext) BufferRequestBody()                         {}
func (m *memoryCaptureTestContext) BufferResponseBody()                        {}
func (m *memoryCaptureTestContext) NeedPauseStreamingResponse()                {}
func (m *memoryCaptureTestContext) PushBuffer(buffer []byte)                   {}
func (m *memoryCaptureTestContext) PopBuffer() []byte                          { return nil }
func (m *memoryCaptureTestContext) BufferQueueSize() int                       { return 0 }
func (m *memoryCaptureTestContext) DisableReroute()                            {}
func (m *memoryCaptureTestContext) SetRequestBodyBufferLimit(byteSize uint32)  {}
func (m *memoryCaptureTestContext) SetResponseBodyBufferLimit(byteSize uint32) {}
func (m *memoryCaptureTestContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (m *memoryCaptureTestContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.EncodeData
}
func (m *memoryCaptureTestContext) HasRequestBody() bool       { return false }
func (m *memoryCaptureTestContext) HasResponseBody() bool      { return true }
func (m *memoryCaptureTestContext) IsWebsocket() bool          { return false }
func (m *memoryCaptureTestContext) IsBinaryRequestBody() bool  { return false }
func (m *memoryCaptureTestContext) IsBinaryResponseBody() bool { return false }

type memoryCaptureNoopLog struct{}

func (memoryCaptureNoopLog) Trace(msg string)                          {}
func (memoryCaptureNoopLog) Tracef(format string, args ...interface{}) {}
func (memoryCaptureNoopLog) Debug(msg string)                          {}
func (memoryCaptureNoopLog) Debugf(format string, args ...interface{}) {}
func (memoryCaptureNoopLog) Info(msg string)                           {}
func (memoryCaptureNoopLog) Infof(format string, args ...interface{})  {}
func (memoryCaptureNoopLog) Warn(msg string)                           {}
func (memoryCaptureNoopLog) Warnf(format string, args ...interface{})  {}
func (memoryCaptureNoopLog) Error(msg string)                          {}
func (memoryCaptureNoopLog) Errorf(format string, args ...interface{}) {}
func (memoryCaptureNoopLog) Critical(msg string)                       {}
func (memoryCaptureNoopLog) Criticalf(format string, args ...interface{}) {
}
func (memoryCaptureNoopLog) ResetID(pluginID string) {}
