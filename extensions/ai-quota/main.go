package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-quota/util"
)

const (
	pluginName = "ai-quota"

	defaultQuotaScope         = "global"
	defaultProvider           = "default"
	defaultTenantHeader       = "x-mse-tenant"
	defaultConsumerHeader     = "x-mse-consumer"
	defaultBalanceKeyTemplate = "billing:balance:{tenant}:{quota_scope}:{consumer}"
	defaultPriceKeyTemplate   = "billing:effective_price:{tenant}:{provider}:{model}:{token_type}"
	defaultAmountScale        = int64(1000000)
	defaultPriceUnitTokens    = int64(1000000)

	ctxQuotaEnabled = "ai-quota-enabled"
	ctxTenant       = "ai-quota-tenant"
	ctxConsumer     = "ai-quota-consumer"
	ctxBalanceKey   = "ai-quota-balance-key"
)

const (
	MissingPolicyDeny  = "deny"
	MissingPolicyAllow = "allow"
	MissingPolicySkip  = "skip"
)

type ChatMode string

const (
	ChatModeCompletion ChatMode = "completion"
	ChatModeNone       ChatMode = "none"
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
	)
}

type QuotaConfig struct {
	redisInfo            RedisInfo `yaml:"redis"`
	QuotaScope           string    `yaml:"quota_scope"`
	Provider             string    `yaml:"provider"`
	TenantHeader         string    `yaml:"tenant_header"`
	ConsumerHeader       string    `yaml:"consumer_header"`
	BalanceKeyTemplate   string    `yaml:"balance_key_template"`
	PriceKeyTemplate     string    `yaml:"price_key_template"`
	AmountScale          int64     `yaml:"amount_scale"`
	PriceUnitTokens      int64     `yaml:"price_unit_tokens"`
	EnablePathSuffixes   []string  `yaml:"enable_path_suffixes"`
	MissingBalancePolicy string    `yaml:"missing_balance_policy"`
	MissingPricePolicy   string    `yaml:"missing_price_policy"`
	MissingUsagePolicy   string    `yaml:"missing_usage_policy"`
	redisClient          wrapper.RedisClient
}

type RedisInfo struct {
	ServiceName string `required:"true" yaml:"service_name" json:"service_name"`
	ServicePort int    `required:"false" yaml:"service_port" json:"service_port"`
	Username    string `required:"false" yaml:"username" json:"username"`
	Password    string `required:"false" yaml:"password" json:"password"`
	Timeout     int    `required:"false" yaml:"timeout" json:"timeout"`
	Database    int    `required:"false" yaml:"database" json:"database"`
}

func parseConfig(json gjson.Result, config *QuotaConfig) error {
	config.QuotaScope = stringDefault(json.Get("quota_scope").String(), defaultQuotaScope)
	config.Provider = stringDefault(json.Get("provider").String(), defaultProvider)
	config.TenantHeader = stringDefault(json.Get("tenant_header").String(), defaultTenantHeader)
	config.ConsumerHeader = stringDefault(json.Get("consumer_header").String(), defaultConsumerHeader)
	config.BalanceKeyTemplate = stringDefault(json.Get("balance_key_template").String(), defaultBalanceKeyTemplate)
	config.PriceKeyTemplate = stringDefault(json.Get("price_key_template").String(), defaultPriceKeyTemplate)
	config.AmountScale = int64Default(json.Get("amount_scale").Int(), defaultAmountScale)
	config.PriceUnitTokens = int64Default(json.Get("price_unit_tokens").Int(), defaultPriceUnitTokens)
	config.MissingBalancePolicy = stringDefault(json.Get("missing_balance_policy").String(), MissingPolicyDeny)
	config.MissingPricePolicy = stringDefault(json.Get("missing_price_policy").String(), MissingPolicySkip)
	config.MissingUsagePolicy = stringDefault(json.Get("missing_usage_policy").String(), MissingPolicySkip)

	if err := validatePolicy(config.MissingBalancePolicy, MissingPolicyDeny, MissingPolicyAllow); err != nil {
		return fmt.Errorf("invalid missing_balance_policy: %w", err)
	}
	if err := validatePolicy(config.MissingPricePolicy, MissingPolicySkip); err != nil {
		return fmt.Errorf("invalid missing_price_policy: %w", err)
	}
	if err := validatePolicy(config.MissingUsagePolicy, MissingPolicySkip); err != nil {
		return fmt.Errorf("invalid missing_usage_policy: %w", err)
	}
	if config.PriceUnitTokens <= 0 {
		return errors.New("price_unit_tokens must be positive")
	}
	if config.AmountScale <= 0 {
		return errors.New("amount_scale must be positive")
	}

	suffixes, err := parsePathSuffixes(json.Get("enable_path_suffixes"))
	if err != nil {
		return err
	}
	config.EnablePathSuffixes = suffixes

	redisConfig := json.Get("redis")
	if !redisConfig.Exists() {
		return errors.New("missing redis in config")
	}
	serviceName := redisConfig.Get("service_name").String()
	if serviceName == "" {
		return errors.New("redis service name must not be empty")
	}
	servicePort := int(redisConfig.Get("service_port").Int())
	if servicePort == 0 {
		if strings.HasSuffix(serviceName, ".static") {
			servicePort = 80
		} else {
			servicePort = 6379
		}
	}
	username := redisConfig.Get("username").String()
	password := redisConfig.Get("password").String()
	timeout := int(redisConfig.Get("timeout").Int())
	if timeout == 0 {
		timeout = 1000
	}
	database := int(redisConfig.Get("database").Int())
	config.redisInfo = RedisInfo{
		ServiceName: serviceName,
		ServicePort: servicePort,
		Username:    username,
		Password:    password,
		Timeout:     timeout,
		Database:    database,
	}
	config.redisClient = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
		FQDN: serviceName,
		Port: int64(servicePort),
	})

	return config.redisClient.Init(username, password, int64(timeout), wrapper.WithDataBase(database))
}

func onHttpRequestHeaders(context wrapper.HttpContext, config QuotaConfig) types.Action {
	context.DisableReroute()

	rawPath := context.Path()
	path, _ := url.Parse(rawPath)
	if !isAIPathEnabled(path.Path, config.EnablePathSuffixes) {
		context.SetContext(ctxQuotaEnabled, false)
		context.DontReadResponseBody()
		return types.ActionContinue
	}
	context.SetContext(ctxQuotaEnabled, true)
	context.DontReadRequestBody()
	context.DontReadResponseBody()

	tenant, _ := proxywasm.GetHttpRequestHeader(config.TenantHeader)
	consumer, _ := proxywasm.GetHttpRequestHeader(config.ConsumerHeader)
	if tenant == "" || consumer == "" {
		return deniedMissingIdentity()
	}

	balanceKey := config.buildBalanceKey(tenant, consumer)
	context.SetContext(ctxTenant, tenant)
	context.SetContext(ctxConsumer, consumer)
	context.SetContext(ctxBalanceKey, balanceKey)
	log.Debugf("ai-quota admission tenant:%s consumer:%s balance_key:%s", tenant, consumer, balanceKey)

	err := config.redisClient.Get(balanceKey, func(response resp.Value) {
		if err := response.Error(); err != nil {
			log.Errorf("ai-quota redis balance read failed: %v", err)
			util.SendResponse(http.StatusServiceUnavailable, "ai-quota.redis_error", "text/plain", "Request denied by ai quota check. Redis balance read failed.")
			return
		}
		if response.IsNull() {
			if config.MissingBalancePolicy == MissingPolicyAllow {
				log.Warnf("ai-quota balance missing, allowing by policy, key:%s", balanceKey)
				proxywasm.ResumeHttpRequest()
				return
			}
			deniedMissingBalance()
			return
		}
		balance, err := redisInteger(response)
		if err != nil {
			log.Errorf("ai-quota invalid balance value for key %s: %v", balanceKey, err)
			util.SendResponse(http.StatusServiceUnavailable, "ai-quota.redis_error", "text/plain", "Request denied by ai quota check. Invalid Redis balance.")
			return
		}
		if balance < 0 {
			deniedNoBalance()
			return
		}
		proxywasm.ResumeHttpRequest()
	})
	if err != nil {
		log.Errorf("ai-quota redis balance dispatch failed: %v", err)
		util.SendResponse(http.StatusServiceUnavailable, "ai-quota.redis_error", "text/plain", "Request denied by ai quota check. Redis balance read failed.")
		return types.ActionContinue
	}
	return types.HeaderStopAllIterationAndWatermark
}

func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config QuotaConfig, data []byte, endOfStream bool) []byte {
	return data
}

func deniedMissingIdentity() types.Action {
	util.SendResponse(http.StatusForbidden, "ai-quota.missing_identity", "text/plain", "Request denied by ai quota check. Missing tenant or consumer identity.")
	return types.ActionContinue
}

func deniedMissingBalance() {
	util.SendResponse(http.StatusForbidden, "ai-quota.missing_balance", "text/plain", "Request denied by ai quota check. Monetary balance is missing.")
}

func deniedNoBalance() {
	util.SendResponse(http.StatusForbidden, "ai-quota.no_balance", "text/plain", "Request denied by ai quota check. No monetary balance left.")
}

func getOperationMode(path string, pathSuffixes []string) ChatMode {
	if isAIPathEnabled(path, pathSuffixes) {
		return ChatModeCompletion
	}
	return ChatModeNone
}

func isAIPathEnabled(requestPath string, enabledSuffixes []string) bool {
	pathWithoutQuery := requestPath
	if parsed, err := url.Parse(requestPath); err == nil && parsed.Path != "" {
		pathWithoutQuery = parsed.Path
	} else if queryPos := strings.Index(requestPath, "?"); queryPos != -1 {
		pathWithoutQuery = requestPath[:queryPos]
	}
	for _, suffix := range enabledSuffixes {
		if strings.HasSuffix(pathWithoutQuery, suffix) {
			return true
		}
	}
	return false
}

func (config QuotaConfig) buildBalanceKey(tenant, consumer string) string {
	replacer := strings.NewReplacer(
		"{tenant}", tenant,
		"{quota_scope}", config.QuotaScope,
		"{consumer}", consumer,
	)
	return replacer.Replace(config.BalanceKeyTemplate)
}

func (config QuotaConfig) buildPriceKey(tenant, model, tokenType string) string {
	replacer := strings.NewReplacer(
		"{tenant}", tenant,
		"{quota_scope}", config.QuotaScope,
		"{consumer}", "",
		"{provider}", config.Provider,
		"{model}", model,
		"{token_type}", tokenType,
	)
	return replacer.Replace(config.PriceKeyTemplate)
}

func parsePathSuffixes(result gjson.Result) ([]string, error) {
	if !result.Exists() {
		return []string{"/v1/chat/completions", "/v1/messages"}, nil
	}
	if !result.IsArray() {
		return nil, errors.New("enable_path_suffixes must be an array")
	}
	values := result.Array()
	suffixes := make([]string, 0, len(values))
	for _, suffix := range values {
		suffixStr := strings.TrimSpace(suffix.String())
		if suffixStr != "" {
			suffixes = append(suffixes, suffixStr)
		}
	}
	if len(suffixes) == 0 {
		return nil, errors.New("enable_path_suffixes must not be empty")
	}
	return suffixes, nil
}

func validatePolicy(policy string, allowed ...string) error {
	for _, value := range allowed {
		if policy == value {
			return nil
		}
	}
	return fmt.Errorf("must be one of %s", strings.Join(allowed, ", "))
}

func redisInteger(response resp.Value) (int64, error) {
	if response.Type() == resp.Integer {
		return int64(response.Integer()), nil
	}
	return strconv.ParseInt(strings.TrimSpace(response.String()), 10, 64)
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func int64Default(value, fallback int64) int64 {
	if value == 0 {
		return fallback
	}
	return value
}
