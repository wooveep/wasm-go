package config

import (
	"fmt"

	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/tidwall/gjson"
)

const (
	MemoryModeOff        = "off"
	MemoryModeRecentOnly = "recent-only"
	MemoryModeDigest     = "digest"
	MemoryModeSemantic   = "semantic"

	FailPolicyOpen = "open"

	InjectRoleSystem    = "system"
	InjectRoleDeveloper = "developer"
)

type RedisStreamConfig struct {
	ServiceName string `yaml:"service_name" json:"service_name"`
	ServicePort int    `yaml:"service_port" json:"service_port"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	Database    int    `yaml:"database" json:"database"`
	Timeout     int    `yaml:"timeout" json:"timeout"`
	Stream      string `yaml:"stream" json:"stream"`
}

type RecentCacheConfig struct {
	ServiceName string `yaml:"service_name" json:"service_name"`
	ServicePort int    `yaml:"service_port" json:"service_port"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	Database    int    `yaml:"database" json:"database"`
	Timeout     int    `yaml:"timeout" json:"timeout"`
	KeyPrefix   string `yaml:"key_prefix" json:"key_prefix"`
}

type ConsoleInternalConfig struct {
	ServiceName  string `yaml:"service_name" json:"service_name"`
	ServicePort  int    `yaml:"service_port" json:"service_port"`
	AssemblePath string `yaml:"assemble_path" json:"assemble_path"`
	TimeoutMS    int    `yaml:"timeout_ms" json:"timeout_ms"`
	AuthToken    string `yaml:"auth_token" json:"auth_token"`
}

type RouteConfig struct {
	MemoryMode        string   `yaml:"memory_mode" json:"memory_mode"`
	RecentWindowTurns int      `yaml:"recent_window_turns" json:"recent_window_turns"`
	MemoryTokenBudget int      `yaml:"memory_token_budget" json:"memory_token_budget"`
	AssembleTimeoutMS int      `yaml:"assemble_timeout_ms" json:"assemble_timeout_ms"`
	InjectRole        string   `yaml:"inject_role" json:"inject_role"`
	SemanticTopK      int      `yaml:"semantic_top_k" json:"semantic_top_k"`
	CaptureResponse   bool     `yaml:"capture_response" json:"capture_response"`
	NoStoreHeader     string   `yaml:"no_store_header" json:"no_store_header"`
	QuestionFrom      string   `yaml:"question_from" json:"question_from"`
	ResponseValueFrom string   `yaml:"response_value_from" json:"response_value_from"`
	StreamValueFrom   string   `yaml:"stream_value_from" json:"stream_value_from"`
	ToolCallsFrom     []string `yaml:"tool_calls_from" json:"tool_calls_from"`
	PolicyVersion     string   `yaml:"policy_version" json:"policy_version"`
}

// PluginConfig is the ai-memory plugin configuration root.
type PluginConfig struct {
	RedisStream        RedisStreamConfig     `yaml:"redis_stream" json:"redis_stream"`
	RecentCache        RecentCacheConfig     `yaml:"recent_cache" json:"recent_cache"`
	ConsoleInternal    ConsoleInternalConfig `yaml:"console_internal" json:"console_internal"`
	TenantHeader       string                `yaml:"tenant_header" json:"tenant_header"`
	ConsumerHeader     string                `yaml:"consumer_header" json:"consumer_header"`
	SessionHeader      string                `yaml:"session_header" json:"session_header"`
	RequestIDHeader    string                `yaml:"request_id_header" json:"request_id_header"`
	EnablePathSuffixes []string              `yaml:"enable_path_suffixes" json:"enable_path_suffixes"`
	FailPolicy         string                `yaml:"fail_policy" json:"fail_policy"`
	Route              RouteConfig           `yaml:"route" json:"route"`

	unsupportedRouteTarget string
}

func (c *PluginConfig) FromJson(json gjson.Result, log log.Log) {
	*c = defaultPluginConfig()
	c.RedisStream = parseRedisStreamConfig(c.RedisStream, json.Get("redis_stream"))
	c.RecentCache = parseRecentCacheConfig(c.RecentCache, json.Get("recent_cache"))
	c.ConsoleInternal = parseConsoleInternalConfig(c.ConsoleInternal, json.Get("console_internal"))
	c.TenantHeader = stringValue(json.Get("tenant_header"), c.TenantHeader)
	c.ConsumerHeader = stringValue(json.Get("consumer_header"), c.ConsumerHeader)
	c.SessionHeader = stringValue(json.Get("session_header"), c.SessionHeader)
	c.RequestIDHeader = stringValue(json.Get("request_id_header"), c.RequestIDHeader)
	c.EnablePathSuffixes = stringSliceValue(json.Get("enable_path_suffixes"), c.EnablePathSuffixes)
	c.FailPolicy = stringValue(json.Get("fail_policy"), c.FailPolicy)
}

func (c *PluginConfig) FromJsonWithGlobal(json gjson.Result, global PluginConfig, log log.Log) {
	*c = global
	c.unsupportedRouteTarget = ""
	for _, key := range []string{"redis_stream", "recent_cache", "console_internal"} {
		if json.Get(key).Exists() {
			c.unsupportedRouteTarget = key
			break
		}
	}
	c.Route = parseRouteConfig(c.Route, json)
}

func (c PluginConfig) Validate() error {
	if c.unsupportedRouteTarget != "" {
		return fmt.Errorf("route-level %s is not supported", c.unsupportedRouteTarget)
	}
	if c.FailPolicy != FailPolicyOpen {
		return fmt.Errorf("unsupported fail_policy %q", c.FailPolicy)
	}
	if c.RedisStream.ServiceName == "" {
		return fmt.Errorf("redis_stream.service_name is required")
	}
	if c.RecentCache.ServiceName == "" {
		return fmt.Errorf("recent_cache.service_name is required")
	}
	if c.ConsoleInternal.ServiceName == "" {
		return fmt.Errorf("console_internal.service_name is required")
	}
	if !validMemoryMode(c.Route.MemoryMode) {
		return fmt.Errorf("unsupported memory_mode %q", c.Route.MemoryMode)
	}
	if c.Route.InjectRole != InjectRoleSystem && c.Route.InjectRole != InjectRoleDeveloper {
		return fmt.Errorf("unsupported inject_role %q", c.Route.InjectRole)
	}
	return nil
}

func (c *PluginConfig) Complete(log log.Log) error {
	return nil
}

func defaultPluginConfig() PluginConfig {
	return PluginConfig{
		RedisStream: RedisStreamConfig{
			ServicePort: 6379,
			Timeout:     500,
			Stream:      "memory:events",
		},
		RecentCache: RecentCacheConfig{
			ServicePort: 6379,
			Timeout:     50,
			KeyPrefix:   "memory:recent",
		},
		ConsoleInternal: ConsoleInternalConfig{
			ServicePort:  80,
			AssemblePath: "/internal/memory/assemble",
			TimeoutMS:    100,
		},
		TenantHeader:       "x-mse-tenant",
		ConsumerHeader:     "x-mse-consumer",
		SessionHeader:      "x-mse-session",
		RequestIDHeader:    "x-request-id",
		EnablePathSuffixes: []string{"/v1/chat/completions", "/v1/messages"},
		FailPolicy:         FailPolicyOpen,
		Route:              defaultRouteConfig(),
	}
}

func defaultRouteConfig() RouteConfig {
	return RouteConfig{
		MemoryMode:        MemoryModeOff,
		InjectRole:        InjectRoleSystem,
		CaptureResponse:   true,
		NoStoreHeader:     "x-higress-ai-memory-no-store",
		QuestionFrom:      `messages.@reverse.#(role=="user").content`,
		ResponseValueFrom: "choices.0.message.content",
		StreamValueFrom:   "choices.0.delta.content",
		ToolCallsFrom: []string{
			"choices.0.message.tool_calls",
			"choices.0.message.function_call",
			"choices.0.delta.tool_calls",
			"choices.0.delta.function_call",
		},
	}
}

func parseRedisStreamConfig(base RedisStreamConfig, json gjson.Result) RedisStreamConfig {
	if !json.Exists() {
		return base
	}
	base.ServiceName = stringValue(json.Get("service_name"), base.ServiceName)
	base.ServicePort = intValue(json.Get("service_port"), base.ServicePort)
	base.Username = stringValue(json.Get("username"), base.Username)
	base.Password = stringValue(json.Get("password"), base.Password)
	base.Database = intValue(json.Get("database"), base.Database)
	base.Timeout = intValue(json.Get("timeout"), base.Timeout)
	base.Stream = stringValue(json.Get("stream"), base.Stream)
	return base
}

func parseRecentCacheConfig(base RecentCacheConfig, json gjson.Result) RecentCacheConfig {
	if !json.Exists() {
		return base
	}
	base.ServiceName = stringValue(json.Get("service_name"), base.ServiceName)
	base.ServicePort = intValue(json.Get("service_port"), base.ServicePort)
	base.Username = stringValue(json.Get("username"), base.Username)
	base.Password = stringValue(json.Get("password"), base.Password)
	base.Database = intValue(json.Get("database"), base.Database)
	base.Timeout = intValue(json.Get("timeout"), base.Timeout)
	base.KeyPrefix = stringValue(json.Get("key_prefix"), base.KeyPrefix)
	return base
}

func parseConsoleInternalConfig(base ConsoleInternalConfig, json gjson.Result) ConsoleInternalConfig {
	if !json.Exists() {
		return base
	}
	base.ServiceName = stringValue(json.Get("service_name"), base.ServiceName)
	base.ServicePort = intValue(json.Get("service_port"), base.ServicePort)
	base.AssemblePath = stringValue(json.Get("assemble_path"), base.AssemblePath)
	base.TimeoutMS = intValue(json.Get("timeout_ms"), base.TimeoutMS)
	base.AuthToken = stringValue(json.Get("auth_token"), base.AuthToken)
	return base
}

func parseRouteConfig(base RouteConfig, json gjson.Result) RouteConfig {
	base.MemoryMode = stringValue(json.Get("memory_mode"), base.MemoryMode)
	base.RecentWindowTurns = intValue(json.Get("recent_window_turns"), base.RecentWindowTurns)
	base.MemoryTokenBudget = intValue(json.Get("memory_token_budget"), base.MemoryTokenBudget)
	base.AssembleTimeoutMS = intValue(json.Get("assemble_timeout_ms"), base.AssembleTimeoutMS)
	base.InjectRole = stringValue(json.Get("inject_role"), base.InjectRole)
	base.SemanticTopK = intValue(json.Get("semantic_top_k"), base.SemanticTopK)
	if json.Get("capture_response").Exists() {
		base.CaptureResponse = json.Get("capture_response").Bool()
	}
	base.NoStoreHeader = stringValue(json.Get("no_store_header"), base.NoStoreHeader)
	base.QuestionFrom = stringValue(json.Get("question_from"), base.QuestionFrom)
	base.ResponseValueFrom = stringValue(json.Get("response_value_from"), base.ResponseValueFrom)
	base.StreamValueFrom = stringValue(json.Get("stream_value_from"), base.StreamValueFrom)
	base.ToolCallsFrom = stringSliceValue(json.Get("tool_calls_from"), base.ToolCallsFrom)
	base.PolicyVersion = stringValue(json.Get("policy_version"), base.PolicyVersion)
	return base
}

func stringValue(value gjson.Result, fallback string) string {
	if !value.Exists() {
		return fallback
	}
	return value.String()
}

func intValue(value gjson.Result, fallback int) int {
	if !value.Exists() {
		return fallback
	}
	return int(value.Int())
}

func stringSliceValue(value gjson.Result, fallback []string) []string {
	if !value.Exists() {
		return fallback
	}
	values := value.Array()
	out := make([]string, 0, len(values))
	for _, item := range values {
		if item.String() == "" {
			continue
		}
		out = append(out, item.String())
	}
	return out
}

func validMemoryMode(value string) bool {
	switch value {
	case MemoryModeOff, MemoryModeRecentOnly, MemoryModeDigest, MemoryModeSemantic:
		return true
	default:
		return false
	}
}
