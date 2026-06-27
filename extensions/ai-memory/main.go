package main

import (
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-memory"

	memoryGateContextKey = "memoryGate"
	maxRequestBodyBytes  = 100 * 1024 * 1024
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseOverrideConfigBy(parseConfig, parseOverrideConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBodyBy(onHttpResponseBody),
	)
}

func parseConfig(json gjson.Result, c *config.PluginConfig, log log.Log) error {
	c.FromJson(json, log)
	if err := c.Validate(); err != nil {
		return err
	}
	return c.Complete(log)
}

func parseOverrideConfig(json gjson.Result, global config.PluginConfig, c *config.PluginConfig, log log.Log) error {
	c.FromJsonWithGlobal(json, global, log)
	if err := c.Validate(); err != nil {
		return err
	}
	return c.Complete(log)
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	ctx.DisableReroute()
	if c.Route.MemoryMode == config.MemoryModeOff {
		markMemoryGate(ctx, "memory-off")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	if contentType == "" || !isJSONContentType(contentType) {
		markMemoryGate(ctx, "unsupported-content-type")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	path := requestPath(ctx)
	if !pathMatchesSuffixes(path, c.EnablePathSuffixes) {
		markMemoryGate(ctx, "unsupported-path")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	tenant, _ := proxywasm.GetHttpRequestHeader(c.TenantHeader)
	if strings.TrimSpace(tenant) == "" {
		markMemoryGate(ctx, "missing-tenant")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	consumer, _ := proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
	if strings.TrimSpace(consumer) == "" {
		markMemoryGate(ctx, "missing-consumer")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	ctx.SetRequestBodyBufferLimit(maxRequestBodyBytes)
	_ = proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	_ = proxywasm.RemoveHttpRequestHeader("Content-Length")
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log) types.Action {
	if memoryGateReason(ctx) != "" {
		return types.ActionContinue
	}
	return types.ActionPause
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	return types.ActionContinue
}

func onHttpResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log log.Log) []byte {
	return chunk
}

func markMemoryGate(ctx wrapper.HttpContext, reason string) {
	ctx.SetContext(memoryGateContextKey, reason)
}

func memoryGateReason(ctx wrapper.HttpContext) string {
	return ctx.GetStringContext(memoryGateContextKey, "")
}

func requestPath(ctx wrapper.HttpContext) string {
	if path := ctx.Path(); path != "" {
		return path
	}
	path, _ := proxywasm.GetHttpRequestHeader(":path")
	return path
}

func isJSONContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func pathMatchesSuffixes(path string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	pathOnly := strings.Split(path, "?")[0]
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(pathOnly, suffix) {
			return true
		}
	}
	return false
}
