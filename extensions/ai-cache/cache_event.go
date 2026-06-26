package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const thinCacheEventKind = "cache_event"
const thinCachePluginVersion = "ai-cache-thin-v1"

type ThinCacheEvent struct {
	sessionctx.EventEnvelope
	SessionID          string            `json:"session_id,omitempty"`
	Route              string            `json:"route"`
	Model              string            `json:"model"`
	RequestPath        string            `json:"request_path"`
	CacheScope         string            `json:"cache_scope"`
	CachePolicyVersion string            `json:"cache_policy_version"`
	StatusCode         int               `json:"status_code"`
	IsStream           bool              `json:"is_stream"`
	ContainsToolCalls  bool              `json:"contains_tool_calls"`
	NoStore            bool              `json:"no_store"`
	Sensitive          bool              `json:"sensitive"`
	PluginVersion      string            `json:"plugin_version"`
	UserContent        string            `json:"user_content,omitempty"`
	AssistantContent   string            `json:"assistant_content,omitempty"`
	FinishReason       string            `json:"finish_reason,omitempty"`
	Usage              *sessionctx.Usage `json:"usage,omitempty"`
}

type thinResponseFacts struct {
	assistantContent  string
	finishReason      string
	usage             sessionctx.Usage
	containsToolCalls bool
	parseFailed       bool
}

type thinStreamCapture struct {
	content           strings.Builder
	buffer            string
	finishReason      string
	usage             sessionctx.Usage
	containsToolCalls bool
	parseFailed       bool
}

func storeThinRequestEventContext(ctx wrapper.HttpContext, body []byte, log logs.Log) {
	ctx.SetContext(CACHE_EVENT_STARTED_AT_KEY, nowMillis())
	ctx.SetContext(CACHE_REQUEST_ID_KEY, currentRequestID())

	request, err := sessionctx.ParseOpenAIChatRequest(body)
	if err != nil {
		log.Warnf("[%s] [storeThinRequestEventContext] parse OpenAI request failed: %v", PLUGIN_NAME, err)
		return
	}
	ctx.SetContext(CACHE_USER_CONTENT_KEY, sessionctx.CurrentUserIntent(request.Messages))
}

func captureThinResponseHeaders(ctx wrapper.HttpContext, log logs.Log) {
	statusHeader, _ := proxywasm.GetHttpResponseHeader(":status")
	statusCode, err := strconv.Atoi(statusHeader)
	if err != nil {
		log.Warnf("[%s] [captureThinResponseHeaders] invalid response status %q: %v", PLUGIN_NAME, statusHeader, err)
		statusCode = 0
	}
	ctx.SetContext(CACHE_RESPONSE_STATUS_KEY, statusCode)
	ctx.SetContext(CACHE_RESPONSE_NOSTORE_KEY, responseHasNoStore())
}

func handleThinResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log logs.Log) {
	if !c.Event.RedisStream.Enabled {
		return
	}

	stream := ctx.GetContext(STREAM_CONTEXT_KEY) != nil
	if stream {
		capture := getThinStreamCapture(ctx)
		if err := capture.appendSSE(unifySSEChunk(chunk)); err != nil {
			log.Warnf("[%s] [handleThinResponseBody] parse streaming response chunk failed: %v", PLUGIN_NAME, err)
			capture.parseFailed = true
		}
		if !isLastChunk {
			return
		}
		emitThinCacheEvent(ctx, c, thinResponseFacts{
			assistantContent:  capture.content.String(),
			finishReason:      capture.finishReason,
			usage:             capture.usage,
			containsToolCalls: capture.containsToolCalls,
			parseFailed:       capture.parseFailed,
		}, true, log)
		return
	}

	if !isLastChunk {
		return
	}
	response, err := sessionctx.ParseOpenAIChatResponse(chunk)
	facts := thinResponseFacts{}
	if err != nil {
		log.Warnf("[%s] [handleThinResponseBody] parse non-streaming response failed: %v", PLUGIN_NAME, err)
		facts.parseFailed = true
	} else {
		facts.assistantContent = response.AssistantContent
		facts.finishReason = sessionctx.ResponseFinishReason(response)
		facts.usage = sessionctx.ResponseUsage(response)
		facts.containsToolCalls = sessionctx.ContainsToolUse(response)
	}
	emitThinCacheEvent(ctx, c, facts, false, log)
}

func getThinStreamCapture(ctx wrapper.HttpContext) *thinStreamCapture {
	if capture, ok := ctx.GetContext(CACHE_STREAM_CAPTURE_KEY).(*thinStreamCapture); ok {
		return capture
	}
	capture := &thinStreamCapture{}
	ctx.SetContext(CACHE_STREAM_CAPTURE_KEY, capture)
	return capture
}

func (c *thinStreamCapture) appendSSE(chunk []byte) error {
	c.buffer += string(chunk)
	for {
		index := strings.IndexAny(c.buffer, "\r\n")
		if index < 0 {
			return nil
		}
		separator := c.buffer[index]
		line := strings.TrimSpace(c.buffer[:index])
		c.buffer = c.buffer[index+1:]
		if separator == '\r' && strings.HasPrefix(c.buffer, "\n") {
			c.buffer = c.buffer[1:]
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content      json.RawMessage `json:"content"`
					ToolCalls    []interface{}   `json:"tool_calls"`
					FunctionCall interface{}     `json:"function_call"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage sessionctx.Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return err
		}
		for _, choice := range event.Choices {
			c.content.WriteString(thinRawTextContent(choice.Delta.Content))
			if len(choice.Delta.ToolCalls) > 0 ||
				choice.Delta.FunctionCall != nil ||
				thinRawContentContainsToolCalls(choice.Delta.Content) ||
				choice.FinishReason == "tool_calls" ||
				choice.FinishReason == "function_call" {
				c.containsToolCalls = true
			}
			if choice.FinishReason != "" {
				c.finishReason = choice.FinishReason
			}
		}
		if !thinUsageEmpty(event.Usage) {
			c.usage = event.Usage
		}
	}
}

func emitThinCacheEvent(ctx wrapper.HttpContext, c config.PluginConfig, facts thinResponseFacts, stream bool, log logs.Log) {
	event := buildThinCacheEvent(ctx, c, facts, stream)
	redisClient := c.GetEventRedisClient()
	if redisClient == nil {
		log.Warnf("[%s] [emitThinCacheEvent] Redis Stream client is not configured", PLUGIN_NAME)
		return
	}

	command, err := sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: c.Event.RedisStream.Stream,
		Field:  c.Event.RedisStream.Field,
		Event:  event,
	})
	if err != nil {
		log.Warnf("[%s] [emitThinCacheEvent] build Redis Stream XADD failed: %v", PLUGIN_NAME, err)
		return
	}
	args := make([]interface{}, len(command))
	for i, arg := range command {
		args[i] = arg
	}
	if err := redisClient.Command(args, func(response resp.Value) {
		if err := response.Error(); err != nil {
			log.Warnf("[%s] [emitThinCacheEvent] Redis Stream XADD failed open: %v", PLUGIN_NAME, err)
		}
	}); err != nil {
		log.Warnf("[%s] [emitThinCacheEvent] Redis Stream XADD dispatch failed open: %v", PLUGIN_NAME, err)
	}
}

func buildThinCacheEvent(ctx wrapper.HttpContext, c config.PluginConfig, facts thinResponseFacts, stream bool) ThinCacheEvent {
	startedAt := thinInt64Context(ctx, CACHE_EVENT_STARTED_AT_KEY, 0)
	endedAt := nowMillis()
	if startedAt <= 0 {
		startedAt = endedAt
	}
	statusCode := thinIntContext(ctx, CACHE_RESPONSE_STATUS_KEY, 0)
	noStore := ctx.GetBoolContext(CACHE_RESPONSE_NOSTORE_KEY, false)
	sensitive := ctx.GetBoolContext(CACHE_SENSITIVE_KEY, false)
	gated := facts.parseFailed || statusCode < 200 || statusCode >= 300 || noStore || sensitive || facts.containsToolCalls

	event := ThinCacheEvent{
		EventEnvelope: sessionctx.NewEventEnvelope(sessionctx.EventEnvelopeInput{
			EventKind:     thinCacheEventKind,
			Tenant:        ctx.GetStringContext(CACHE_TENANT_CONTEXT_KEY, ""),
			Consumer:      ctx.GetStringContext(CACHE_CONSUMER_CONTEXT_KEY, ""),
			RequestID:     ctx.GetStringContext(CACHE_REQUEST_ID_KEY, ""),
			RequestDigest: ctx.GetStringContext(CACHE_DIGEST_CONTEXT_KEY, ""),
			StartedAtMS:   startedAt,
			EndedAtMS:     endedAt,
		}),
		SessionID:          ctx.GetStringContext(CACHE_SESSION_CONTEXT_KEY, ""),
		Route:              ctx.GetStringContext(CACHE_ROUTE_CONTEXT_KEY, ""),
		Model:              ctx.GetStringContext(CACHE_MODEL_CONTEXT_KEY, ""),
		RequestPath:        ctx.GetStringContext(CACHE_PATH_CONTEXT_KEY, ""),
		CacheScope:         c.CacheScope,
		CachePolicyVersion: c.CachePolicyVersion,
		StatusCode:         statusCode,
		IsStream:           stream,
		ContainsToolCalls:  facts.containsToolCalls,
		NoStore:            noStore,
		Sensitive:          sensitive,
		PluginVersion:      thinCachePluginVersion,
		FinishReason:       facts.finishReason,
	}
	if !thinUsageEmpty(facts.usage) {
		usage := facts.usage
		event.Usage = &usage
	}
	if !gated {
		event.UserContent = ctx.GetStringContext(CACHE_USER_CONTENT_KEY, "")
		event.AssistantContent = facts.assistantContent
	} else if !sensitive && !noStore {
		event.UserContent = ctx.GetStringContext(CACHE_USER_CONTENT_KEY, "")
	}
	return event
}

func currentRequestID() string {
	if requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id"); requestID != "" {
		return requestID
	}
	property, _ := proxywasm.GetProperty([]string{"x_request_id"})
	return string(property)
}

func responseHasNoStore() bool {
	cacheControl, _ := proxywasm.GetHttpResponseHeader("cache-control")
	for _, directive := range strings.Split(cacheControl, ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
			return true
		}
	}
	return false
}

func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func thinIntContext(ctx wrapper.HttpContext, key string, fallback int) int {
	switch value := ctx.GetContext(key).(type) {
	case int:
		return value
	case int64:
		return int(value)
	default:
		return fallback
	}
}

func thinInt64Context(ctx wrapper.HttpContext, key string, fallback int64) int64 {
	switch value := ctx.GetContext(key).(type) {
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return fallback
	}
}

func thinUsageEmpty(usage sessionctx.Usage) bool {
	return usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0
}

func thinRawTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return thinTextContent(value)
}

func thinTextContent(value interface{}) string {
	switch content := value.(type) {
	case string:
		return content
	case []interface{}:
		var parts []string
		for _, item := range content {
			object, ok := item.(map[string]interface{})
			if !ok || object["type"] != "text" {
				continue
			}
			text, _ := object["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func thinRawContentContainsToolCalls(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var content struct {
		ToolCalls []interface{} `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		return false
	}
	return len(content.ToolCalls) > 0
}
