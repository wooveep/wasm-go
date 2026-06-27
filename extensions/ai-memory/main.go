package main

import (
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-memory"

	memoryGateContextKey          = "memoryGate"
	memoryTenantContextKey        = "memoryTenant"
	memoryConsumerContextKey      = "memoryConsumer"
	memorySessionContextKey       = "memorySession"
	memoryRequestIDContextKey     = "memoryRequestID"
	memoryRouteContextKey         = "memoryRoute"
	memoryModelContextKey         = "memoryModel"
	memoryRequestPathContextKey   = "memoryRequestPath"
	memoryModeContextKey          = "memoryMode"
	memoryPolicyVersionContextKey = "memoryPolicyVersion"
	memoryStartedAtContextKey     = "memoryStartedAt"
	memoryStreamContextKey        = "memoryStream"
	memoryNoStoreContextKey       = "memoryNoStore"
	memoryRequestDigestContextKey = "memoryRequestDigest"

	maxRequestBodyBytes = 100 * 1024 * 1024
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
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		markMemoryGate(ctx, "missing-tenant")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	consumer, _ := proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
	consumer = strings.TrimSpace(consumer)
	if consumer == "" {
		markMemoryGate(ctx, "missing-consumer")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	session, _ := proxywasm.GetHttpRequestHeader(c.SessionHeader)
	requestID := currentRequestID(c.RequestIDHeader)
	ctx.SetContext(memoryTenantContextKey, tenant)
	ctx.SetContext(memoryConsumerContextKey, consumer)
	ctx.SetContext(memorySessionContextKey, strings.TrimSpace(session))
	ctx.SetContext(memoryRequestIDContextKey, requestID)
	ctx.SetContext(memoryRouteContextKey, requestRoute())
	ctx.SetContext(memoryRequestPathContextKey, path)
	ctx.SetContext(memoryModeContextKey, c.Route.MemoryMode)
	ctx.SetContext(memoryPolicyVersionContextKey, c.Route.PolicyVersion)
	ctx.SetContext(memoryStartedAtContextKey, nowMillis())
	ctx.SetContext(memoryNoStoreContextKey, requestHasNoStore(c.Route.NoStoreHeader))
	ctx.SetRequestBodyBufferLimit(maxRequestBodyBytes)
	_ = proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	_ = proxywasm.RemoveHttpRequestHeader("Content-Length")
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log) types.Action {
	if memoryGateReason(ctx) != "" {
		return types.ActionContinue
	}
	request, err := sessionctx.ParseOpenAIChatRequest(body)
	if err != nil {
		markMemoryGate(ctx, "request-parse-failed")
		return types.ActionContinue
	}
	digest, err := sessionctx.BuildOpenAIChatRequestDigest(request)
	if err != nil {
		markMemoryGate(ctx, "request-digest-failed")
		return types.ActionContinue
	}
	ctx.SetContext(memoryModelContextKey, request.Model)
	ctx.SetContext(memoryStreamContextKey, request.Stream)
	ctx.SetContext(memoryRequestDigestContextKey, digest)
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

func requestRoute() string {
	routeName, err := proxywasm.GetProperty([]string{"route_name"})
	if err != nil || len(routeName) == 0 {
		return "-"
	}
	return string(routeName)
}

func currentRequestID(header string) string {
	if requestID, _ := proxywasm.GetHttpRequestHeader(header); strings.TrimSpace(requestID) != "" {
		return strings.TrimSpace(requestID)
	}
	property, _ := proxywasm.GetProperty([]string{"x_request_id"})
	return string(property)
}

func requestHasNoStore(header string) bool {
	if value, _ := proxywasm.GetHttpRequestHeader(header); isTruthyHeaderValue(value) {
		return true
	}
	cacheControl, _ := proxywasm.GetHttpRequestHeader("cache-control")
	for _, directive := range strings.Split(cacheControl, ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
			return true
		}
	}
	return false
}

func isTruthyHeaderValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "true", "yes":
		return true
	default:
		return false
	}
}

func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
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
