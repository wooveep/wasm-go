package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/test"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmtest "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestQwenNativeStreamCompletesOpenAIResponse(t *testing.T) {
	wasmtest.RunGoTest(t, func(t *testing.T) {
		bootstrap, status := wasmtest.NewTestHost(json.RawMessage(`{"provider":{"type":"generic","genericHost":"http://127.0.0.1:1","apiTokens":["fixture"]}}`))
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer bootstrap.Reset()

		const initial = `event: result
:HTTP_STATUS/200
data: {"request_id":"fixture-request","output":{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"null"}]},"usage":{"input_tokens":13,"output_tokens":1,"total_tokens":14}}

`
		const upstream = initial + `event: result
:HTTP_STATUS/200
data: {"request_id":"fixture-request","output":{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}]},"usage":{"input_tokens":13,"output_tokens":1,"total_tokens":14}}

`
		for _, split := range []int{1, len(upstream) / 2, len(upstream)} {
			t.Run(fmt.Sprintf("split-%d", split), func(t *testing.T) {
				var pc config.PluginConfig
				pc.FromJson(gjson.Parse(`{"provider":{"type":"qwen","apiTokens":["fixture"],"qwenEnableCompatible":false}}`))
				require.NoError(t, pc.Validate())
				require.NoError(t, pc.Complete())
				ctx := test.NewMockHttpContext()
				ctx.SetContext(provider.CtxKeyApiName, provider.ApiNameChatCompletion)
				var output strings.Builder
				output.Write(OnStreamingResponseBodyForTest(ctx, pc, []byte(upstream[:split]), false))
				output.Write(OnStreamingResponseBodyForTest(ctx, pc, []byte(upstream[split:]), true))
				require.Contains(t, output.String(), `"finish_reason":"stop"`)
				require.Contains(t, output.String(), `"total_tokens":14`)
				require.Equal(t, 1, strings.Count(output.String(), "data: [DONE]\n\n"))
				require.True(t, strings.HasSuffix(output.String(), "data: [DONE]\n\n"))
			})
		}
		t.Run("truncated-upstream-does-not-complete", func(t *testing.T) {
			var pc config.PluginConfig
			pc.FromJson(gjson.Parse(`{"provider":{"type":"qwen","apiTokens":["fixture"],"qwenEnableCompatible":false}}`))
			require.NoError(t, pc.Validate())
			require.NoError(t, pc.Complete())
			ctx := test.NewMockHttpContext()
			ctx.SetContext(provider.CtxKeyApiName, provider.ApiNameChatCompletion)
			output := OnStreamingResponseBodyForTest(ctx, pc, []byte(initial), true)
			require.Contains(t, string(output), `"content":"OK"`)
			require.NotContains(t, string(output), `"finish_reason":"stop"`)
			require.NotContains(t, string(output), "[DONE]")
		})

	})
}
