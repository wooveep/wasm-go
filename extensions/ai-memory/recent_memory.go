package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const (
	memoryRecentSchemaVersion = 1
	maxRecentRecordBytes      = 256 * 1024
	maxRecentMessageBytes     = 64 * 1024
)

var memoryLoadRecentMemory = loadRecentMemory

type memoryRecentRecord struct {
	SchemaVersion int                   `json:"schema_version"`
	Tenant        string                `json:"tenant"`
	Consumer      string                `json:"consumer"`
	PolicyVersion string                `json:"policy_version"`
	Messages      []memoryRecentMessage `json:"messages"`
	ExpiresAtMS   int64                 `json:"expires_at_ms"`
}

type memoryRecentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type memoryRecentValidationInput struct {
	Tenant        string
	Consumer      string
	PolicyVersion string
	NowMS         int64
}

func loadRecentMemory(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) error {
	key := buildRecentMemoryKey(
		c.RecentCache.KeyPrefix,
		ctx.GetStringContext(memoryTenantContextKey, ""),
		ctx.GetStringContext(memoryConsumerContextKey, ""),
		ctx.GetStringContext(memorySessionContextKey, ""),
	)
	client := wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
		FQDN: c.RecentCache.ServiceName,
		Port: int64(c.RecentCache.ServicePort),
	})
	if err := client.Init(
		c.RecentCache.Username,
		c.RecentCache.Password,
		int64(c.RecentCache.Timeout),
		wrapper.WithDataBase(c.RecentCache.Database),
		wrapper.WithDisableBuffer(),
	); err != nil {
		return err
	}
	return client.Get(key, func(response resp.Value) {
		handleRecentMemoryResponse(key, response, ctx, c, log)
	})
}

func handleRecentMemoryResponse(key string, response resp.Value, ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) {
	if err := response.Error(); err != nil {
		log.Warnf("[ai-memory] recent memory lookup rejected for key %s: %v", key, err)
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	if response.IsNull() {
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	record, err := loadAndValidateRecentMemory(response.String(), memoryRecentValidationInput{
		Tenant:        ctx.GetStringContext(memoryTenantContextKey, ""),
		Consumer:      ctx.GetStringContext(memoryConsumerContextKey, ""),
		PolicyVersion: c.Route.PolicyVersion,
		NowMS:         nowMillis(),
	})
	if err != nil {
		log.Warnf("[ai-memory] recent memory record rejected for key %s: %v", key, err)
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	ctx.SetContext(memoryRecentMessagesContextKey, record.toOpenAIMessages())
	continueAfterRecentMemory(ctx, c, log)
}

func continueAfterRecentMemory(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) {
	if shouldUseMemoryAssemble(c) {
		if err := dispatchMemoryAssemble(ctx, c, log); err != nil {
			log.Warnf("[ai-memory] Console assemble dispatch failed open: %v", err)
			proxywasm.ResumeHttpRequest()
		}
		return
	}
	proxywasm.ResumeHttpRequest()
}

func buildRecentMemoryKey(prefix, tenant, consumer, session string) string {
	prefix = strings.TrimRight(prefix, ":")
	if prefix == "" {
		prefix = "memory:recent"
	}
	return strings.Join([]string{
		prefix,
		defaultKeyPart(tenant),
		defaultKeyPart(consumer),
		defaultKeyPart(session),
	}, ":")
}

func defaultKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func loadAndValidateRecentMemory(raw string, input memoryRecentValidationInput) (memoryRecentRecord, error) {
	if strings.TrimSpace(raw) == "" {
		return memoryRecentRecord{}, errors.New("empty recent memory record")
	}
	if len(raw) > maxRecentRecordBytes {
		return memoryRecentRecord{}, errors.New("recent memory record too large")
	}
	var record memoryRecentRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return memoryRecentRecord{}, errors.New("recent memory record is not valid JSON")
	}
	if err := validateRecentMemory(record, input); err != nil {
		return memoryRecentRecord{}, err
	}
	return record, nil
}

func validateRecentMemory(record memoryRecentRecord, input memoryRecentValidationInput) error {
	if record.SchemaVersion != memoryRecentSchemaVersion {
		return errors.New("unsupported recent memory schema version")
	}
	if record.Tenant != input.Tenant {
		return errors.New("recent memory tenant mismatch")
	}
	if record.Consumer != input.Consumer {
		return errors.New("recent memory consumer mismatch")
	}
	if record.PolicyVersion != input.PolicyVersion {
		return errors.New("recent memory policy mismatch")
	}
	if record.ExpiresAtMS <= 0 || record.ExpiresAtMS <= input.NowMS {
		return errors.New("recent memory record expired")
	}
	for _, message := range record.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return fmt.Errorf("unsupported recent memory role %q", message.Role)
		}
		if len(message.Content) > maxRecentMessageBytes {
			return errors.New("recent memory message too large")
		}
	}
	return nil
}

func (r memoryRecentRecord) toOpenAIMessages() []sessionctx.OpenAIMessage {
	messages := make([]sessionctx.OpenAIMessage, 0, len(r.Messages))
	for _, message := range r.Messages {
		messages = append(messages, sessionctx.OpenAIMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return messages
}
