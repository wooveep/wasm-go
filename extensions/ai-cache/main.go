// 这个文件中主要将OnHttpRequestHeaders、OnHttpRequestBody、OnHttpResponseHeaders、OnHttpResponseBody这四个函数实现
// 其中的缓存思路调用cache.go中的逻辑，然后cache.go中的逻辑会调用textEmbeddingProvider和vectorStoreProvider中的逻辑（实例）
package main

import (
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	PLUGIN_NAME                 = "ai-cache"
	CACHE_KEY_CONTEXT_KEY       = "cacheKey"
	CACHE_KEY_EMBEDDING_KEY     = "cacheKeyEmbedding"
	CACHE_CONTENT_CONTEXT_KEY   = "cacheContent"
	CACHE_GATE_CONTEXT_KEY      = "cacheGate"
	PARTIAL_MESSAGE_CONTEXT_KEY = "partialMessage"
	TOOL_CALLS_CONTEXT_KEY      = "toolCalls"
	STREAM_CONTEXT_KEY          = "stream"
	SKIP_CACHE_HEADER           = "x-higress-skip-ai-cache"
	CACHE_TENANT_CONTEXT_KEY    = "cacheTenant"
	CACHE_CONSUMER_CONTEXT_KEY  = "cacheConsumer"
	CACHE_SESSION_CONTEXT_KEY   = "cacheSession"
	CACHE_PATH_CONTEXT_KEY      = "cacheRequestPath"
	CACHE_ROUTE_CONTEXT_KEY     = "cacheRoute"
	CACHE_MODEL_CONTEXT_KEY     = "cacheModel"
	CACHE_DIGEST_CONTEXT_KEY    = "cacheRequestDigest"
	CACHE_MATERIALIZED_KEY      = "cacheMaterializedKey"
	ERROR_PARTIAL_MESSAGE_KEY   = "errorPartialMessage"

	DEFAULT_MAX_BODY_BYTES uint32 = 100 * 1024 * 1024
)

func main() {}

func init() {
	// CreateClient()
	wrapper.SetCtx(
		PLUGIN_NAME,
		wrapper.ParseOverrideConfigBy(parseConfig, parseOverrideConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBodyBy(onHttpResponseBody),
	)
}

func parseConfig(json gjson.Result, c *config.PluginConfig, log log.Log) error {
	// config.EmbeddingProviderConfig.FromJson(json.Get("embeddingProvider"))
	// config.VectorDatabaseProviderConfig.FromJson(json.Get("vectorBaseProvider"))
	// config.RedisConfig.FromJson(json.Get("redis"))
	c.FromJson(json, log)
	if err := c.Validate(); err != nil {
		return err
	}
	// Note that initializing the client during the parseConfig phase may cause errors, such as Redis not being usable in Docker Compose.
	if err := c.Complete(log); err != nil {
		log.Errorf("complete config failed: %v", err)
		return err
	}
	return nil
}

func parseOverrideConfig(json gjson.Result, global config.PluginConfig, c *config.PluginConfig, log log.Log) error {
	c.FromJsonWithGlobal(json, global, log)
	if err := c.Validate(); err != nil {
		return err
	}
	if err := c.Complete(log); err != nil {
		log.Errorf("complete override config failed: %v", err)
		return err
	}
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	ctx.DisableReroute()
	skipCache, _ := proxywasm.GetHttpRequestHeader(SKIP_CACHE_HEADER)
	if isTruthyHeaderValue(skipCache) {
		ctx.SetContext(SKIP_CACHE_HEADER, struct{}{})
		markCacheGate(ctx, "skip-header")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	// The request does not have a body.
	if contentType == "" {
		return types.ActionContinue
	}
	if !isJSONContentType(contentType) {
		log.Warnf("content is not json, can't process: %s", contentType)
		markCacheGate(ctx, "unsupported-content-type")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	if c.HasThinConfig() {
		if requestHasNoStore() {
			markCacheGate(ctx, "no-store")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		if c.RoutePolicy.EnableBypass {
			markCacheGate(ctx, "route-bypass")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		path := requestPath(ctx)
		if !pathMatchesSuffixes(path, c.RoutePolicy.EnabledPathSuffixes) {
			markCacheGate(ctx, "unsupported-path")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		tenant, _ := proxywasm.GetHttpRequestHeader(c.TenantHeader)
		if strings.TrimSpace(tenant) == "" {
			markCacheGate(ctx, "missing-tenant")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		consumer, _ := proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
		if c.CacheScope == config.CACHE_SCOPE_CONSUMER && strings.TrimSpace(consumer) == "" {
			markCacheGate(ctx, "missing-consumer")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		ctx.SetContext(CACHE_TENANT_CONTEXT_KEY, tenant)
		ctx.SetContext(CACHE_CONSUMER_CONTEXT_KEY, consumer)
		session, _ := proxywasm.GetHttpRequestHeader(c.SessionHeader)
		ctx.SetContext(CACHE_SESSION_CONTEXT_KEY, session)
		ctx.SetContext(CACHE_PATH_CONTEXT_KEY, path)
	}
	ctx.SetRequestBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
	_ = proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	// The request has a body and requires delaying the header transmission until a cache miss occurs,
	// at which point the header should be sent.
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log) types.Action {
	if cacheGateReason(ctx) != "" {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	bodyJson := gjson.ParseBytes(body)
	// TODO: It may be necessary to support stream mode determination for different LLM providers.
	stream := false
	if bodyJson.Get("stream").Bool() {
		stream = true
		ctx.SetContext(STREAM_CONTEXT_KEY, struct{}{})
	}

	if c.HasThinConfig() {
		return onThinHttpRequestBody(ctx, c, body, log, stream)
	}

	var key string
	if c.CacheKeyStrategy == config.CACHE_KEY_STRATEGY_LAST_QUESTION {
		log.Debugf("[onHttpRequestBody] cache key strategy is last question, cache key from: %s", c.CacheKeyFrom)
		key = bodyJson.Get(c.CacheKeyFrom).String()
	} else if c.CacheKeyStrategy == config.CACHE_KEY_STRATEGY_ALL_QUESTIONS {
		log.Debugf("[onHttpRequestBody] cache key strategy is all questions, cache key from: messages")
		messages := bodyJson.Get("messages").Array()
		var userMessages []string
		for _, msg := range messages {
			if msg.Get("role").String() == "user" {
				userMessages = append(userMessages, msg.Get("content").String())
			}
		}
		key = strings.Join(userMessages, "\n")
	} else if c.CacheKeyStrategy == config.CACHE_KEY_STRATEGY_DISABLED {
		log.Info("[onHttpRequestBody] cache key strategy is disabled")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	} else {
		log.Warnf("[onHttpRequestBody] unknown cache key strategy: %s", c.CacheKeyStrategy)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	ctx.SetContext(CACHE_KEY_CONTEXT_KEY, key)
	log.Debugf("[onHttpRequestBody] key: %s", key)
	if key == "" {
		log.Debug("[onHttpRequestBody] parse key from request body failed")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	if err := CheckCacheForKey(key, ctx, c, log, stream, true); err != nil {
		log.Errorf("[onHttpRequestBody] check cache for key: %s failed, error: %v", key, err)
		return types.ActionContinue
	}

	return types.ActionPause
}

func onThinHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log, stream bool) types.Action {
	model, requestDigest, err := BuildOpenAIRequestDigest(body)
	if err != nil {
		log.Warnf("[onThinHttpRequestBody] build request digest failed, fail open: %v", err)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	material, err := BuildScopedCacheKeyMaterial(ScopedCacheKeyInput{
		KeyPrefix:          c.MaterializedLookup.Redis.KeyPrefix,
		Tenant:             ctx.GetStringContext(CACHE_TENANT_CONTEXT_KEY, ""),
		Consumer:           ctx.GetStringContext(CACHE_CONSUMER_CONTEXT_KEY, ""),
		CacheScope:         c.CacheScope,
		Route:              requestRoute(),
		Model:              model,
		RequestDigest:      requestDigest,
		CachePolicyVersion: c.CachePolicyVersion,
	})
	if err != nil {
		log.Warnf("[onThinHttpRequestBody] build materialized cache key failed, fail open: %v", err)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	ctx.SetContext(CACHE_ROUTE_CONTEXT_KEY, material.Route)
	ctx.SetContext(CACHE_MODEL_CONTEXT_KEY, material.Model)
	ctx.SetContext(CACHE_DIGEST_CONTEXT_KEY, material.RequestDigest)
	ctx.SetContext(CACHE_MATERIALIZED_KEY, material.RedisKey)

	if !c.MaterializedLookup.Redis.Enabled || !c.RoutePolicy.EnableRedisLookup {
		log.Debug("[onThinHttpRequestBody] materialized Redis lookup is disabled, fail open")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	if err := CheckMaterializedCacheForKey(material, ctx, c, log, stream); err != nil {
		log.Errorf("[onThinHttpRequestBody] materialized Redis lookup failed for key: %s, error: %v", material.RedisKey, err)
		return types.ActionContinue
	}

	return types.ActionPause
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	if cacheGateReason(ctx) != "" || ctx.GetContext(SKIP_CACHE_HEADER) != nil {
		ctx.SetUserAttribute("cache_status", "skip")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	if ctx.GetContext(CACHE_KEY_CONTEXT_KEY) != nil {
		ctx.SetUserAttribute("cache_status", "miss")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
	}
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	if strings.Contains(contentType, "text/event-stream") {
		ctx.SetContext(STREAM_CONTEXT_KEY, struct{}{})
	} else {
		ctx.SetResponseBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
	}

	return types.ActionContinue
}

func markCacheGate(ctx wrapper.HttpContext, reason string) {
	ctx.SetContext(CACHE_GATE_CONTEXT_KEY, reason)
}

func cacheGateReason(ctx wrapper.HttpContext) string {
	return ctx.GetStringContext(CACHE_GATE_CONTEXT_KEY, "")
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

func requestHasNoStore() bool {
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

func onHttpResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log log.Log) []byte {
	log.Debugf("[onHttpResponseBody] is last chunk: %v", isLastChunk)
	log.Debugf("[onHttpResponseBody] chunk: %s", string(chunk))

	if ctx.GetContext(TOOL_CALLS_CONTEXT_KEY) != nil || ctx.GetContext(ERROR_PARTIAL_MESSAGE_KEY) != nil {
		return chunk
	}

	key := ctx.GetContext(CACHE_KEY_CONTEXT_KEY)
	if key == nil {
		log.Debug("[onHttpResponseBody] key is nil, skip cache")
		return chunk
	}

	stream := ctx.GetContext(STREAM_CONTEXT_KEY)
	var err error
	if !isLastChunk {
		if stream == nil {
			err = handleNonStreamChunk(ctx, c, chunk, log)
		} else {
			err = handleStreamChunk(ctx, c, unifySSEChunk(chunk), log)
		}
		if err != nil {
			log.Errorf("[onHttpResponseBody] handle non last chunk failed, error: %v", err)
			// Set an empty struct in the context to indicate an error in processing the partial message
			ctx.SetContext(ERROR_PARTIAL_MESSAGE_KEY, struct{}{})
		}
		return chunk
	}
	var value string
	if stream == nil {
		value, err = processNonStreamLastChunk(ctx, c, chunk, log)
	} else {
		value, err = processStreamLastChunk(ctx, c, unifySSEChunk(chunk), log)
	}

	if err != nil {
		log.Errorf("[onHttpResponseBody] process last chunk failed, error: %v", err)
		return chunk
	}

	cacheResponse(ctx, c, key.(string), value, log)
	uploadEmbeddingAndAnswer(ctx, c, key.(string), value, log)
	return chunk
}
