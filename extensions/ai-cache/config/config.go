package config

import (
	"fmt"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/cache"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/embedding"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/vector"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/tidwall/gjson"
)

const (
	CACHE_KEY_STRATEGY_LAST_QUESTION = "lastQuestion"
	CACHE_KEY_STRATEGY_ALL_QUESTIONS = "allQuestions"
	CACHE_KEY_STRATEGY_DISABLED      = "disabled"

	CACHE_SCOPE_TENANT   = "tenant"
	CACHE_SCOPE_CONSUMER = "consumer"

	FAIL_POLICY_OPEN   = "open"
	FAIL_POLICY_CLOSED = "closed"
)

type MaterializedLookupConfig struct {
	Redis RedisEndpointConfig
}

type RedisEndpointConfig struct {
	Enabled     bool
	ServiceName string
	ServicePort int
	KeyPrefix   string
	Timeout     int
}

type ConsoleLookupConfig struct {
	Enabled     bool
	ServiceName string
	ServicePort int
	Path        string
	Timeout     int
}

type EventConfig struct {
	RedisStream RedisStreamConfig
}

type RedisStreamConfig struct {
	Enabled     bool
	ServiceName string
	ServicePort int
	Stream      string
	Field       string
	Timeout     int
}

type RoutePolicyConfig struct {
	EnableRedisLookup   bool
	EnableConsoleLookup bool
	EnableReplay        bool
	EnableBypass        bool
	EnabledPathSuffixes []string
}

type PluginConfig struct {
	// @Title zh-CN 返回 HTTP 响应的模版
	// @Description zh-CN 用 %s 标记需要被 cache value 替换的部分
	ResponseTemplate string
	// @Title zh-CN 返回流式 HTTP 响应的模版
	// @Description zh-CN 用 %s 标记需要被 cache value 替换的部分
	StreamResponseTemplate string

	cacheProvider     cache.Provider
	embeddingProvider embedding.Provider
	vectorProvider    vector.Provider

	embeddingProviderConfig *embedding.ProviderConfig
	vectorProviderConfig    *vector.ProviderConfig
	cacheProviderConfig     *cache.ProviderConfig

	CacheKeyFrom         string
	CacheValueFrom       string
	CacheStreamValueFrom string
	CacheToolCallsFrom   string

	// @Title zh-CN 启用语义化缓存
	// @Description zh-CN 控制是否启用语义化缓存功能。true 表示启用，false 表示禁用。
	EnableSemanticCache bool

	// @Title zh-CN 缓存键策略
	// @Description zh-CN 决定如何生成缓存键的策略。可选值: "lastQuestion" (使用最后一个问题), "allQuestions" (拼接所有问题) 或 "disabled" (禁用缓存)
	CacheKeyStrategy string

	MaterializedLookup MaterializedLookupConfig
	ConsoleLookup      ConsoleLookupConfig
	Event              EventConfig
	RoutePolicy        RoutePolicyConfig
	TenantHeader       string
	ConsumerHeader     string
	CacheScope         string
	CachePolicyVersion string
	FailPolicy         string
}

func (c *PluginConfig) FromJson(json gjson.Result, log log.Log) {
	c.embeddingProviderConfig = &embedding.ProviderConfig{}
	c.vectorProviderConfig = &vector.ProviderConfig{}
	c.cacheProviderConfig = &cache.ProviderConfig{}
	c.vectorProviderConfig.FromJson(json.Get("vector"))
	c.embeddingProviderConfig.FromJson(json.Get("embedding"))
	c.cacheProviderConfig.FromJson(json.Get("cache"))
	if json.Get("redis").Exists() {
		// compatible with legacy config
		c.cacheProviderConfig.ConvertLegacyJson(json)
	}

	c.CacheKeyStrategy = json.Get("cacheKeyStrategy").String()
	if c.CacheKeyStrategy == "" {
		c.CacheKeyStrategy = CACHE_KEY_STRATEGY_LAST_QUESTION // set default value
	}
	c.CacheKeyFrom = json.Get("cacheKeyFrom").String()
	if c.CacheKeyFrom == "" {
		c.CacheKeyFrom = "messages.@reverse.0.content"
	}
	c.CacheValueFrom = json.Get("cacheValueFrom").String()
	if c.CacheValueFrom == "" {
		c.CacheValueFrom = "choices.0.message.content"
	}
	c.CacheStreamValueFrom = json.Get("cacheStreamValueFrom").String()
	if c.CacheStreamValueFrom == "" {
		c.CacheStreamValueFrom = "choices.0.delta.content"
	}
	c.CacheToolCallsFrom = json.Get("cacheToolCallsFrom").String()
	if c.CacheToolCallsFrom == "" {
		c.CacheToolCallsFrom = "choices.0.delta.content.tool_calls"
	}

	c.StreamResponseTemplate = json.Get("streamResponseTemplate").String()
	if c.StreamResponseTemplate == "" {
		c.StreamResponseTemplate = `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}` + "\n\ndata:[DONE]\n\n"
	}
	c.ResponseTemplate = json.Get("responseTemplate").String()
	if c.ResponseTemplate == "" {
		c.ResponseTemplate = `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
	}

	if json.Get("enableSemanticCache").Exists() {
		c.EnableSemanticCache = json.Get("enableSemanticCache").Bool()
	} else if c.GetVectorProvider() == nil {
		c.EnableSemanticCache = false // set value to false when no vector provider
	} else {
		c.EnableSemanticCache = true // set default value to true
	}

	c.MaterializedLookup.Redis = RedisEndpointConfig{
		Enabled:     json.Get("materialized_lookup.redis.enabled").Bool(),
		ServiceName: json.Get("materialized_lookup.redis.service_name").String(),
		ServicePort: int(json.Get("materialized_lookup.redis.service_port").Int()),
		KeyPrefix:   json.Get("materialized_lookup.redis.key_prefix").String(),
		Timeout:     int(json.Get("materialized_lookup.redis.timeout").Int()),
	}
	c.ConsoleLookup = ConsoleLookupConfig{
		Enabled:     json.Get("console_lookup.enabled").Bool(),
		ServiceName: json.Get("console_lookup.service_name").String(),
		ServicePort: int(json.Get("console_lookup.service_port").Int()),
		Path:        json.Get("console_lookup.path").String(),
		Timeout:     int(json.Get("console_lookup.timeout").Int()),
	}
	if c.ConsoleLookup.Path == "" {
		c.ConsoleLookup.Path = "/internal/cache/lookup"
	}
	c.Event.RedisStream = RedisStreamConfig{
		Enabled:     json.Get("redis_stream.enabled").Bool(),
		ServiceName: json.Get("redis_stream.service_name").String(),
		ServicePort: int(json.Get("redis_stream.service_port").Int()),
		Stream:      json.Get("redis_stream.stream").String(),
		Field:       json.Get("redis_stream.field").String(),
		Timeout:     int(json.Get("redis_stream.timeout").Int()),
	}
	if c.Event.RedisStream.Field == "" {
		c.Event.RedisStream.Field = "event"
	}
	if c.Event.RedisStream.Stream == "" {
		c.Event.RedisStream.Stream = "cache:events"
	}
	c.RoutePolicy = RoutePolicyConfig{
		EnableRedisLookup:   true,
		EnableConsoleLookup: c.ConsoleLookup.Enabled,
		EnableReplay:        true,
		EnableBypass:        false,
		EnabledPathSuffixes: jsonStringArray(json.Get("route_policy.enabled_path_suffixes")),
	}
	if json.Get("route_policy.enable_redis_lookup").Exists() {
		c.RoutePolicy.EnableRedisLookup = json.Get("route_policy.enable_redis_lookup").Bool()
	}
	if json.Get("route_policy.enable_console_lookup").Exists() {
		c.RoutePolicy.EnableConsoleLookup = json.Get("route_policy.enable_console_lookup").Bool()
	}
	if json.Get("route_policy.enable_replay").Exists() {
		c.RoutePolicy.EnableReplay = json.Get("route_policy.enable_replay").Bool()
	}
	if json.Get("route_policy.enable_bypass").Exists() {
		c.RoutePolicy.EnableBypass = json.Get("route_policy.enable_bypass").Bool()
	}
	c.TenantHeader = json.Get("tenant_header").String()
	if c.TenantHeader == "" {
		c.TenantHeader = "x-mse-tenant"
	}
	c.ConsumerHeader = json.Get("consumer_header").String()
	if c.ConsumerHeader == "" {
		c.ConsumerHeader = "x-mse-consumer"
	}
	c.CacheScope = json.Get("cache_scope").String()
	if c.CacheScope == "" {
		c.CacheScope = CACHE_SCOPE_TENANT
	}
	c.CachePolicyVersion = json.Get("cache_policy_version").String()
	c.FailPolicy = json.Get("fail_policy").String()
	if c.FailPolicy == "" {
		c.FailPolicy = FAIL_POLICY_OPEN
	}

	// compatible with legacy config
	convertLegacyMapFields(c, json, log)
}

func (c *PluginConfig) FromJsonWithGlobal(json gjson.Result, global PluginConfig, log log.Log) {
	if !global.hasProviderConfigs() {
		c.FromJson(json, log)
		return
	}
	*c = global
	c.applyProviderOverrides(json)
	consoleLookupEnabledOverride := json.Get("console_lookup.enabled").Exists()
	routePolicyConsoleOverride := json.Get("route_policy.enable_console_lookup").Exists()
	if json.Get("cacheKeyStrategy").Exists() {
		c.CacheKeyStrategy = json.Get("cacheKeyStrategy").String()
	}
	if json.Get("cacheKeyFrom").Exists() {
		c.CacheKeyFrom = json.Get("cacheKeyFrom").String()
	}
	if json.Get("cacheValueFrom").Exists() {
		c.CacheValueFrom = json.Get("cacheValueFrom").String()
	}
	if json.Get("cacheStreamValueFrom").Exists() {
		c.CacheStreamValueFrom = json.Get("cacheStreamValueFrom").String()
	}
	if json.Get("cacheToolCallsFrom").Exists() {
		c.CacheToolCallsFrom = json.Get("cacheToolCallsFrom").String()
	}
	if json.Get("responseTemplate").Exists() {
		c.ResponseTemplate = json.Get("responseTemplate").String()
	}
	if json.Get("streamResponseTemplate").Exists() {
		c.StreamResponseTemplate = json.Get("streamResponseTemplate").String()
	}
	if json.Get("enableSemanticCache").Exists() {
		c.EnableSemanticCache = json.Get("enableSemanticCache").Bool()
	}
	c.MaterializedLookup.Redis = mergeRedisEndpoint(c.MaterializedLookup.Redis, json.Get("materialized_lookup.redis"))
	c.ConsoleLookup = mergeConsoleLookup(c.ConsoleLookup, json.Get("console_lookup"))
	c.Event.RedisStream = mergeRedisStream(c.Event.RedisStream, json.Get("redis_stream"))
	c.RoutePolicy = mergeRoutePolicy(c.RoutePolicy, json.Get("route_policy"))
	if consoleLookupEnabledOverride && !routePolicyConsoleOverride {
		c.RoutePolicy.EnableConsoleLookup = c.ConsoleLookup.Enabled
	}
	if json.Get("tenant_header").Exists() && json.Get("tenant_header").String() != "" {
		c.TenantHeader = json.Get("tenant_header").String()
	}
	if json.Get("consumer_header").Exists() && json.Get("consumer_header").String() != "" {
		c.ConsumerHeader = json.Get("consumer_header").String()
	}
	if json.Get("cache_scope").Exists() {
		c.CacheScope = json.Get("cache_scope").String()
	}
	if json.Get("cache_policy_version").Exists() {
		c.CachePolicyVersion = json.Get("cache_policy_version").String()
	}
	if json.Get("fail_policy").Exists() {
		c.FailPolicy = json.Get("fail_policy").String()
	}
	convertLegacyMapFields(c, json, log)
}

func (c *PluginConfig) hasProviderConfigs() bool {
	return c.embeddingProviderConfig != nil &&
		c.vectorProviderConfig != nil &&
		c.cacheProviderConfig != nil
}

func (c *PluginConfig) applyProviderOverrides(json gjson.Result) {
	if json.Get("embedding").Exists() {
		c.embeddingProviderConfig = &embedding.ProviderConfig{}
		c.embeddingProviderConfig.FromJson(json.Get("embedding"))
		c.embeddingProvider = nil
	}
	if json.Get("vector").Exists() {
		c.vectorProviderConfig = &vector.ProviderConfig{}
		c.vectorProviderConfig.FromJson(json.Get("vector"))
		c.vectorProvider = nil
	}
	if json.Get("cache").Exists() || json.Get("redis").Exists() {
		c.cacheProviderConfig = &cache.ProviderConfig{}
		c.cacheProviderConfig.FromJson(json.Get("cache"))
		if json.Get("redis").Exists() {
			c.cacheProviderConfig.ConvertLegacyJson(json)
		}
		c.cacheProvider = nil
	}
}

func (c *PluginConfig) Validate() error {
	// if cache provider is configured, validate it
	if c.cacheProviderConfig.GetProviderType() != "" {
		if err := c.cacheProviderConfig.Validate(); err != nil {
			return err
		}
	}
	if c.embeddingProviderConfig.GetProviderType() != "" {
		if err := c.embeddingProviderConfig.Validate(); err != nil {
			return err
		}
	}
	if c.vectorProviderConfig.GetProviderType() != "" {
		if err := c.vectorProviderConfig.Validate(); err != nil {
			return err
		}
	}

	// cache, vector, and embedding cannot all be empty
	if c.vectorProviderConfig.GetProviderType() == "" &&
		c.embeddingProviderConfig.GetProviderType() == "" &&
		c.cacheProviderConfig.GetProviderType() == "" &&
		!c.HasThinConfig() {
		return fmt.Errorf("vector, embedding and cache provider cannot be all empty")
	}

	// Validate the value of CacheKeyStrategy
	if c.CacheKeyStrategy != CACHE_KEY_STRATEGY_LAST_QUESTION &&
		c.CacheKeyStrategy != CACHE_KEY_STRATEGY_ALL_QUESTIONS &&
		c.CacheKeyStrategy != CACHE_KEY_STRATEGY_DISABLED {
		return fmt.Errorf("invalid CacheKeyStrategy: %s", c.CacheKeyStrategy)
	}
	if c.CacheScope != CACHE_SCOPE_TENANT && c.CacheScope != CACHE_SCOPE_CONSUMER {
		return fmt.Errorf("invalid cache_scope: %s", c.CacheScope)
	}
	if c.FailPolicy != FAIL_POLICY_OPEN && c.FailPolicy != FAIL_POLICY_CLOSED {
		return fmt.Errorf("invalid fail_policy: %s", c.FailPolicy)
	}
	if c.HasThinConfig() && c.CachePolicyVersion == "" {
		return fmt.Errorf("cache_policy_version is required when thin cache behavior is enabled")
	}
	if c.MaterializedLookup.Redis.Enabled {
		if err := validateRedisEndpoint(c.MaterializedLookup.Redis); err != nil {
			return err
		}
	}
	if c.ConsoleLookup.Enabled {
		if err := validateConsoleLookup(c.ConsoleLookup); err != nil {
			return err
		}
	}
	if c.Event.RedisStream.Enabled {
		if err := validateRedisStream(c.Event.RedisStream); err != nil {
			return err
		}
	}

	// If semantic cache is enabled, ensure necessary components are configured
	// if c.EnableSemanticCache {
	// 	if c.embeddingProviderConfig.GetProviderType() == "" {
	// 		return fmt.Errorf("semantic cache is enabled but embedding provider is not configured")
	// 	}
	// 	// if only configure cache, just warn the user
	// }
	return nil
}

func (c *PluginConfig) HasThinConfig() bool {
	return c.MaterializedLookup.Redis.Enabled ||
		c.ConsoleLookup.Enabled ||
		c.Event.RedisStream.Enabled
}

func (c *PluginConfig) Complete(log log.Log) error {
	var err error
	if c.embeddingProviderConfig.GetProviderType() != "" {
		log.Debugf("embedding provider is set to %s", c.embeddingProviderConfig.GetProviderType())
		c.embeddingProvider, err = embedding.CreateProvider(*c.embeddingProviderConfig)
		if err != nil {
			return err
		}
	} else {
		log.Info("embedding provider is not configured")
		c.embeddingProvider = nil
	}
	if c.cacheProviderConfig.GetProviderType() != "" {
		log.Debugf("cache provider is set to %s", c.cacheProviderConfig.GetProviderType())
		c.cacheProvider, err = cache.CreateProvider(*c.cacheProviderConfig, log)
		if err != nil {
			return err
		}
	} else {
		log.Info("cache provider is not configured")
		c.cacheProvider = nil
	}
	if c.vectorProviderConfig.GetProviderType() != "" {
		log.Debugf("vector provider is set to %s", c.vectorProviderConfig.GetProviderType())
		c.vectorProvider, err = vector.CreateProvider(*c.vectorProviderConfig)
		if err != nil {
			return err
		}
	} else {
		log.Info("vector provider is not configured")
		c.vectorProvider = nil
	}
	return nil
}

func (c *PluginConfig) GetEmbeddingProvider() embedding.Provider {
	return c.embeddingProvider
}

func (c *PluginConfig) GetVectorProvider() vector.Provider {
	return c.vectorProvider
}

func (c *PluginConfig) GetVectorProviderConfig() vector.ProviderConfig {
	return *c.vectorProviderConfig
}

func (c *PluginConfig) GetCacheProvider() cache.Provider {
	return c.cacheProvider
}

func convertLegacyMapFields(c *PluginConfig, json gjson.Result, log log.Log) {
	keyMap := map[string]string{
		"cacheKeyFrom.requestBody":         "cacheKeyFrom",
		"cacheValueFrom.requestBody":       "cacheValueFrom",
		"cacheStreamValueFrom.requestBody": "cacheStreamValueFrom",
		"returnResponseTemplate":           "responseTemplate",
		"returnStreamResponseTemplate":     "streamResponseTemplate",
	}

	for oldKey, newKey := range keyMap {
		if json.Get(oldKey).Exists() {
			log.Debugf("[convertLegacyMapFields] mapping %s to %s", oldKey, newKey)
			setField(c, newKey, json.Get(oldKey).String(), log)
		} else {
			log.Debugf("[convertLegacyMapFields] %s not exists", oldKey)
		}
	}
}

func jsonStringArray(value gjson.Result) []string {
	if !value.Exists() {
		return nil
	}
	items := value.Array()
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.String() == "" {
			continue
		}
		out = append(out, item.String())
	}
	return out
}

func mergeRedisEndpoint(base RedisEndpointConfig, value gjson.Result) RedisEndpointConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("service_name").Exists() {
		base.ServiceName = value.Get("service_name").String()
	}
	if value.Get("service_port").Exists() {
		base.ServicePort = int(value.Get("service_port").Int())
	}
	if value.Get("key_prefix").Exists() {
		base.KeyPrefix = value.Get("key_prefix").String()
	}
	if value.Get("timeout").Exists() {
		base.Timeout = int(value.Get("timeout").Int())
	}
	return base
}

func mergeConsoleLookup(base ConsoleLookupConfig, value gjson.Result) ConsoleLookupConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("service_name").Exists() {
		base.ServiceName = value.Get("service_name").String()
	}
	if value.Get("service_port").Exists() {
		base.ServicePort = int(value.Get("service_port").Int())
	}
	if value.Get("path").Exists() {
		base.Path = value.Get("path").String()
	}
	if value.Get("timeout").Exists() {
		base.Timeout = int(value.Get("timeout").Int())
	}
	return base
}

func mergeRedisStream(base RedisStreamConfig, value gjson.Result) RedisStreamConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("service_name").Exists() {
		base.ServiceName = value.Get("service_name").String()
	}
	if value.Get("service_port").Exists() {
		base.ServicePort = int(value.Get("service_port").Int())
	}
	if value.Get("stream").Exists() {
		base.Stream = value.Get("stream").String()
	}
	if value.Get("field").Exists() {
		base.Field = value.Get("field").String()
	}
	if value.Get("timeout").Exists() {
		base.Timeout = int(value.Get("timeout").Int())
	}
	return base
}

func mergeRoutePolicy(base RoutePolicyConfig, value gjson.Result) RoutePolicyConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enable_redis_lookup").Exists() {
		base.EnableRedisLookup = value.Get("enable_redis_lookup").Bool()
	}
	if value.Get("enable_console_lookup").Exists() {
		base.EnableConsoleLookup = value.Get("enable_console_lookup").Bool()
	}
	if value.Get("enable_replay").Exists() {
		base.EnableReplay = value.Get("enable_replay").Bool()
	}
	if value.Get("enable_bypass").Exists() {
		base.EnableBypass = value.Get("enable_bypass").Bool()
	}
	if value.Get("enabled_path_suffixes").Exists() {
		base.EnabledPathSuffixes = jsonStringArray(value.Get("enabled_path_suffixes"))
	}
	return base
}

func validateRedisEndpoint(cfg RedisEndpointConfig) error {
	if cfg.ServiceName == "" {
		return fmt.Errorf("materialized_lookup.redis.service_name is required when enabled")
	}
	if cfg.ServicePort <= 0 {
		return fmt.Errorf("materialized_lookup.redis.service_port must be positive when enabled")
	}
	if cfg.KeyPrefix == "" {
		return fmt.Errorf("materialized_lookup.redis.key_prefix is required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("materialized_lookup.redis.timeout must be positive when enabled")
	}
	return nil
}

func validateConsoleLookup(cfg ConsoleLookupConfig) error {
	if cfg.ServiceName == "" {
		return fmt.Errorf("console_lookup.service_name is required when enabled")
	}
	if cfg.ServicePort <= 0 {
		return fmt.Errorf("console_lookup.service_port must be positive when enabled")
	}
	if cfg.Path == "" {
		return fmt.Errorf("console_lookup.path is required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("console_lookup.timeout must be positive when enabled")
	}
	return nil
}

func validateRedisStream(cfg RedisStreamConfig) error {
	if cfg.ServiceName == "" {
		return fmt.Errorf("redis_stream.service_name is required when enabled")
	}
	if cfg.ServicePort <= 0 {
		return fmt.Errorf("redis_stream.service_port must be positive when enabled")
	}
	if cfg.Stream == "" {
		return fmt.Errorf("redis_stream.stream is required when enabled")
	}
	if cfg.Field == "" {
		return fmt.Errorf("redis_stream.field is required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("redis_stream.timeout must be positive when enabled")
	}
	return nil
}

func setField(c *PluginConfig, fieldName string, value string, log log.Log) {
	switch fieldName {
	case "cacheKeyFrom":
		c.CacheKeyFrom = value
	case "cacheValueFrom":
		c.CacheValueFrom = value
	case "cacheStreamValueFrom":
		c.CacheStreamValueFrom = value
	case "responseTemplate":
		c.ResponseTemplate = value
	case "streamResponseTemplate":
		c.StreamResponseTemplate = value
	}
	log.Debugf("[setField] set %s to %s", fieldName, value)
}
