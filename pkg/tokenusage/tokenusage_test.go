package tokenusage

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/stretchr/testify/require"
)

func TestProviderUsageMergesAcrossStreamingCallbacks(t *testing.T) {
	ctx := newTokenUsageTestContext()

	usage := GetTokenUsage(ctx, []byte(`data: {"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":2}}}`+"\n\n"))
	require.Equal(t, map[string]any{
		"prompt_tokens": float64(5),
		"prompt_tokens_details": map[string]any{
			"cached_tokens": float64(2),
		},
	}, usage.ProviderUsage)

	usage = GetTokenUsage(ctx, []byte(`data: {"model":"gpt-4","usage":{"completion_tokens":8,"prompt_tokens_details":{"audio_tokens":1},"completion_tokens_details":{"reasoning_tokens":3}}}`+"\n\n"))

	require.Equal(t, map[string]any{
		"prompt_tokens": float64(5),
		"prompt_tokens_details": map[string]any{
			"cached_tokens": float64(2),
			"audio_tokens":  float64(1),
		},
		"completion_tokens": float64(8),
		"completion_tokens_details": map[string]any{
			"reasoning_tokens": float64(3),
		},
	}, usage.ProviderUsage)
}

type tokenUsageTestContext struct {
	values     map[string]any
	attributes map[string]any
}

func newTokenUsageTestContext() *tokenUsageTestContext {
	return &tokenUsageTestContext{
		values:     map[string]any{},
		attributes: map[string]any{},
	}
}

func (c *tokenUsageTestContext) Scheme() string { return "" }
func (c *tokenUsageTestContext) Host() string   { return "" }
func (c *tokenUsageTestContext) Path() string   { return "" }
func (c *tokenUsageTestContext) Method() string { return "" }

func (c *tokenUsageTestContext) SetContext(key string, value any) {
	c.values[key] = value
}

func (c *tokenUsageTestContext) GetContext(key string) any {
	return c.values[key]
}

func (c *tokenUsageTestContext) GetBoolContext(key string, defaultValue bool) bool {
	if value, ok := c.values[key].(bool); ok {
		return value
	}
	return defaultValue
}

func (c *tokenUsageTestContext) GetStringContext(key, defaultValue string) string {
	if value, ok := c.values[key].(string); ok {
		return value
	}
	return defaultValue
}

func (c *tokenUsageTestContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	if value, ok := c.values[key].([]byte); ok {
		return value
	}
	return defaultValue
}

func (c *tokenUsageTestContext) GetUserAttribute(key string) any {
	return c.attributes[key]
}

func (c *tokenUsageTestContext) SetUserAttribute(key string, value any) {
	c.attributes[key] = value
}

func (c *tokenUsageTestContext) SetUserAttributeMap(kvmap map[string]any) {
	c.attributes = kvmap
}

func (c *tokenUsageTestContext) GetUserAttributeMap() map[string]any {
	return c.attributes
}

func (c *tokenUsageTestContext) WriteUserAttributeToLog() error                  { return nil }
func (c *tokenUsageTestContext) WriteUserAttributeToLogWithKey(key string) error { return nil }
func (c *tokenUsageTestContext) WriteUserAttributeToTrace() error                { return nil }
func (c *tokenUsageTestContext) DontReadRequestBody()                            {}
func (c *tokenUsageTestContext) DontReadResponseBody()                           {}
func (c *tokenUsageTestContext) BufferRequestBody()                              {}
func (c *tokenUsageTestContext) BufferResponseBody()                             {}
func (c *tokenUsageTestContext) NeedPauseStreamingResponse()                     {}
func (c *tokenUsageTestContext) PushBuffer(buffer []byte)                        {}
func (c *tokenUsageTestContext) PopBuffer() []byte                               { return nil }
func (c *tokenUsageTestContext) BufferQueueSize() int                            { return 0 }
func (c *tokenUsageTestContext) DisableReroute()                                 {}
func (c *tokenUsageTestContext) SetRequestBodyBufferLimit(byteSize uint32)       {}
func (c *tokenUsageTestContext) SetResponseBodyBufferLimit(byteSize uint32)      {}
func (c *tokenUsageTestContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (c *tokenUsageTestContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.EncodeData
}
func (c *tokenUsageTestContext) HasRequestBody() bool       { return false }
func (c *tokenUsageTestContext) HasResponseBody() bool      { return false }
func (c *tokenUsageTestContext) IsWebsocket() bool          { return false }
func (c *tokenUsageTestContext) IsBinaryRequestBody() bool  { return false }
func (c *tokenUsageTestContext) IsBinaryResponseBody() bool { return false }
