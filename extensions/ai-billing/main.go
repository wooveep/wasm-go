package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-billing"

	defaultQuotaScope     = "global"
	defaultProvider       = "default"
	defaultTenantHeader   = "x-mse-tenant"
	defaultConsumerHeader = "x-mse-consumer"
	defaultBillingPath    = "/billing/events"
	defaultTimeout        = uint32(500)

	FailPolicyOpen = "open"

	ctxBillingEnabled = "ai-billing-enabled"
	ctxStartTime      = "ai-billing-start-time"
	ctxRequestPath    = "ai-billing-request-path"
	ctxRequestID      = "ai-billing-request-id"
	ctxTenant         = "ai-billing-tenant"
	ctxConsumer       = "ai-billing-consumer"
	ctxRoute          = "ai-billing-route"
	ctxCluster        = "ai-billing-cluster"
	ctxStatusCode     = "ai-billing-status-code"
	ctxPriceVersion   = "ai-billing-price-version"
	ctxInputToken     = "ai-billing-input-token"
	ctxOutputToken    = "ai-billing-output-token"
	ctxTotalToken     = "ai-billing-total-token"
	ctxModel          = "ai-billing-model"
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingResponseBody),
		wrapper.ProcessResponseBody(onHttpResponseBody),
	)
}

type BillingConfig struct {
	BillingService     BillingService `yaml:"billing_service"`
	QuotaScope         string         `yaml:"quota_scope"`
	Provider           string         `yaml:"provider"`
	TenantHeader       string         `yaml:"tenant_header"`
	ConsumerHeader     string         `yaml:"consumer_header"`
	EnablePathSuffixes []string       `yaml:"enable_path_suffixes"`
	FailPolicy         string         `yaml:"fail_policy"`
	httpClient         wrapper.HttpClient
}

type BillingService struct {
	ServiceName string `yaml:"service_name" json:"service_name"`
	ServicePort int    `yaml:"service_port" json:"service_port"`
	Path        string `yaml:"path" json:"path"`
	Timeout     uint32 `yaml:"timeout" json:"timeout"`
	AuthToken   string `yaml:"auth_token" json:"auth_token"`
}

type BillingEvent struct {
	RequestID             string `json:"request_id"`
	IdempotencyKey        string `json:"idempotency_key"`
	Tenant                string `json:"tenant"`
	Consumer              string `json:"consumer"`
	QuotaScope            string `json:"quota_scope"`
	Provider              string `json:"provider"`
	Model                 string `json:"model"`
	Route                 string `json:"route"`
	Cluster               string `json:"cluster"`
	RequestPath           string `json:"request_path"`
	StatusCode            int    `json:"status_code"`
	StartTimeMs           int64  `json:"start_time_ms"`
	EndTimeMs             int64  `json:"end_time_ms"`
	IsStream              bool   `json:"is_stream"`
	InputTokens           int64  `json:"input_tokens"`
	OutputTokens          int64  `json:"output_tokens"`
	TotalTokens           int64  `json:"total_tokens"`
	UsageMissing          bool   `json:"usage_missing"`
	PriceVersion          string `json:"price_version,omitempty"`
	GatewayCalculatedCost *int64 `json:"gateway_calculated_cost,omitempty"`
}

func parseConfig(configJson gjson.Result, config *BillingConfig) error {
	config.QuotaScope = stringDefault(configJson.Get("quota_scope").String(), defaultQuotaScope)
	config.Provider = stringDefault(configJson.Get("provider").String(), defaultProvider)
	config.TenantHeader = stringDefault(configJson.Get("tenant_header").String(), defaultTenantHeader)
	config.ConsumerHeader = stringDefault(configJson.Get("consumer_header").String(), defaultConsumerHeader)
	config.FailPolicy = stringDefault(configJson.Get("fail_policy").String(), FailPolicyOpen)
	if config.FailPolicy != FailPolicyOpen {
		return errors.New("fail_policy only supports open")
	}
	suffixes, err := parsePathSuffixes(configJson.Get("enable_path_suffixes"))
	if err != nil {
		return err
	}
	config.EnablePathSuffixes = suffixes

	service := configJson.Get("billing_service")
	if !service.Exists() {
		return errors.New("missing billing_service in config")
	}
	serviceName := service.Get("service_name").String()
	if serviceName == "" {
		return errors.New("billing_service.service_name must not be empty")
	}
	servicePort := int(service.Get("service_port").Int())
	if servicePort == 0 {
		servicePort = 80
	}
	path := stringDefault(service.Get("path").String(), defaultBillingPath)
	timeout := uint32(service.Get("timeout").Uint())
	if timeout == 0 {
		timeout = defaultTimeout
	}
	config.BillingService = BillingService{
		ServiceName: serviceName,
		ServicePort: servicePort,
		Path:        path,
		Timeout:     timeout,
		AuthToken:   service.Get("auth_token").String(),
	}
	config.httpClient = wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: serviceName,
		Port: int64(servicePort),
	})
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
	requestPath := ctx.Path()
	if !isAIPathEnabled(requestPath, config.EnablePathSuffixes) {
		ctx.SetContext(ctxBillingEnabled, false)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	ctx.SetContext(ctxBillingEnabled, true)
	ctx.SetContext(ctxStartTime, time.Now().UnixMilli())
	ctx.SetContext(ctxRequestPath, requestPath)

	requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id")
	if requestID == "" {
		requestID = stringProperty([]string{"x_request_id"}, "")
	}
	ctx.SetContext(ctxRequestID, requestID)

	if tenant, _ := proxywasm.GetHttpRequestHeader(config.TenantHeader); tenant != "" {
		ctx.SetContext(ctxTenant, tenant)
	}
	if consumer, _ := proxywasm.GetHttpRequestHeader(config.ConsumerHeader); consumer != "" {
		ctx.SetContext(ctxConsumer, consumer)
	}
	if priceVersion, _ := proxywasm.GetHttpRequestHeader("x-ai-price-version"); priceVersion != "" {
		ctx.SetContext(ctxPriceVersion, priceVersion)
	}
	ctx.SetContext(ctxRoute, stringProperty([]string{"route_name"}, "-"))
	ctx.SetContext(ctxCluster, stringProperty([]string{"cluster_name"}, "-"))
	return types.ActionContinue
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return types.ActionContinue
	}
	status, _ := proxywasm.GetHttpResponseHeader(":status")
	statusCode, err := strconv.Atoi(status)
	if err != nil {
		statusCode = http.StatusBadGateway
	}
	ctx.SetContext(ctxStatusCode, statusCode)

	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	if !strings.Contains(contentType, "text/event-stream") {
		ctx.BufferResponseBody()
	}
	return types.ActionContinue
}

func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config BillingConfig, data []byte, endOfStream bool) []byte {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return data
	}
	recordUsage(ctx, data)
	if endOfStream {
		deliverBillingEvent(ctx, config, true)
	}
	return data
}

func onHttpResponseBody(ctx wrapper.HttpContext, config BillingConfig, body []byte) types.Action {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return types.ActionContinue
	}
	recordUsage(ctx, body)
	deliverBillingEvent(ctx, config, false)
	return types.ActionContinue
}

func recordUsage(ctx wrapper.HttpContext, body []byte) {
	usage := tokenusage.GetTokenUsage(ctx, body)
	if usage.TotalToken <= 0 {
		return
	}
	ctx.SetContext(ctxInputToken, usage.InputToken)
	ctx.SetContext(ctxOutputToken, usage.OutputToken)
	ctx.SetContext(ctxTotalToken, usage.TotalToken)
	ctx.SetContext(ctxModel, usage.Model)
}

func deliverBillingEvent(ctx wrapper.HttpContext, config BillingConfig, isStream bool) {
	event := buildBillingEvent(ctx, config, isStream)
	body, err := json.Marshal(event)
	if err != nil {
		log.Errorf("ai-billing marshal event failed: %v", err)
		return
	}
	headers := [][2]string{{"content-type", "application/json"}}
	err = config.httpClient.Post(config.BillingService.Path, headers, body, func(statusCode int, _ http.Header, _ []byte) {
		if statusCode >= 500 || statusCode == http.StatusBadGateway {
			log.Warnf("ai-billing delivery failed open, status:%d request_id:%s", statusCode, event.RequestID)
			return
		}
		log.Debugf("ai-billing delivery accepted, status:%d request_id:%s", statusCode, event.RequestID)
	}, config.BillingService.Timeout)
	if err != nil {
		log.Warnf("ai-billing dispatch failed open, request_id:%s err:%v", event.RequestID, err)
	}
}

func buildBillingEvent(ctx wrapper.HttpContext, config BillingConfig, isStream bool) BillingEvent {
	inputTokens := int64FromContext(ctx.GetContext(ctxInputToken))
	outputTokens := int64FromContext(ctx.GetContext(ctxOutputToken))
	totalTokens := int64FromContext(ctx.GetContext(ctxTotalToken))
	usageMissing := totalTokens <= 0
	requestID := ctx.GetStringContext(ctxRequestID, "")
	if requestID == "" {
		requestID = fmt.Sprintf("%s:%s:%d", ctx.GetStringContext(ctxTenant, ""), ctx.GetStringContext(ctxConsumer, ""), intFromContext(ctx.GetContext(ctxStatusCode)))
	}
	event := BillingEvent{
		RequestID:      requestID,
		IdempotencyKey: requestID,
		Tenant:         ctx.GetStringContext(ctxTenant, ""),
		Consumer:       ctx.GetStringContext(ctxConsumer, ""),
		QuotaScope:     config.QuotaScope,
		Provider:       config.Provider,
		Model:          ctx.GetStringContext(ctxModel, tokenusage.ModelUnknown),
		Route:          ctx.GetStringContext(ctxRoute, "-"),
		Cluster:        ctx.GetStringContext(ctxCluster, "-"),
		RequestPath:    ctx.GetStringContext(ctxRequestPath, ""),
		StatusCode:     intDefault(intFromContext(ctx.GetContext(ctxStatusCode)), http.StatusBadGateway),
		StartTimeMs:    int64FromContext(ctx.GetContext(ctxStartTime)),
		EndTimeMs:      time.Now().UnixMilli(),
		IsStream:       isStream,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		TotalTokens:    totalTokens,
		UsageMissing:   usageMissing,
		PriceVersion:   ctx.GetStringContext(ctxPriceVersion, ""),
	}
	return event
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

func statusCodeFromHeaders(headers [][2]string) int {
	for _, header := range headers {
		if header[0] != ":status" {
			continue
		}
		statusCode, err := strconv.Atoi(header[1])
		if err != nil {
			return http.StatusBadGateway
		}
		return statusCode
	}
	return http.StatusBadGateway
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

func stringProperty(path []string, fallback string) string {
	raw, err := proxywasm.GetProperty(path)
	if err != nil || len(raw) == 0 {
		return fallback
	}
	return string(raw)
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func int64FromContext(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func intFromContext(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func intDefault(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
