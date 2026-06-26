package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const materializedRecordSchemaVersion = "ai-cache.materialized.v1"

type MaterializedReplayRecord struct {
	SchemaVersion       string            `json:"schema_version"`
	Tenant              string            `json:"tenant"`
	Consumer            string            `json:"consumer,omitempty"`
	Route               string            `json:"route"`
	Model               string            `json:"model"`
	CacheScope          string            `json:"cache_scope"`
	CachePolicyVersion  string            `json:"cache_policy_version"`
	RequestDigest       string            `json:"request_digest"`
	SoftExpiresAt       int64             `json:"soft_expires_at"`
	HardExpiresAt       int64             `json:"hard_expires_at"`
	Response            json.RawMessage   `json:"response"`
	Usage               json.RawMessage   `json:"usage,omitempty"`
	FinishReason        string            `json:"finish_reason,omitempty"`
	StreamReplayable    bool              `json:"stream_replayable,omitempty"`
	StreamChunks        []json.RawMessage `json:"stream_chunks,omitempty"`
	TrustedUpstreamFact *bool             `json:"upstream_invoked,omitempty"`
}

func LoadAndValidateMaterializedRecord(raw string, material ScopedCacheKeyMaterial, stream bool) (MaterializedReplayRecord, error) {
	if strings.TrimSpace(raw) == "" {
		return MaterializedReplayRecord{}, errors.New("empty materialized record")
	}

	var record MaterializedReplayRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return MaterializedReplayRecord{}, errors.New("materialized record is not valid JSON")
	}
	if err := validateMaterializedRecord(record, material, stream, time.Now().Unix()); err != nil {
		return MaterializedReplayRecord{}, err
	}
	return record, nil
}

func validateMaterializedRecord(record MaterializedReplayRecord, material ScopedCacheKeyMaterial, stream bool, now int64) error {
	if record.SchemaVersion != materializedRecordSchemaVersion {
		return errors.New("unsupported materialized record schema version")
	}
	if record.CacheScope != material.CacheScope {
		return errors.New("materialized record cache scope mismatch")
	}
	if record.Tenant != material.Tenant {
		return errors.New("materialized record tenant mismatch")
	}
	if material.CacheScope == config.CACHE_SCOPE_CONSUMER && record.Consumer != material.Consumer {
		return errors.New("materialized record consumer mismatch")
	}
	if record.Route != material.Route {
		return errors.New("materialized record route mismatch")
	}
	if record.Model != material.Model {
		return errors.New("materialized record model mismatch")
	}
	if record.CachePolicyVersion != material.CachePolicyVersion {
		return errors.New("materialized record policy version mismatch")
	}
	if record.RequestDigest != material.RequestDigest {
		return errors.New("materialized record request digest mismatch")
	}
	if record.SoftExpiresAt <= 0 || record.SoftExpiresAt <= now {
		return errors.New("materialized record soft expiration is expired")
	}
	if record.HardExpiresAt <= 0 || record.HardExpiresAt <= now {
		return errors.New("materialized record hard expiration is expired")
	}
	if record.FinishReason == "" {
		return errors.New("materialized record finish reason is required")
	}
	if err := requireJSONObject(record.Usage, "usage"); err != nil {
		return err
	}
	if stream {
		if !record.StreamReplayable {
			return errors.New("materialized record is not stream replayable")
		}
		if len(record.StreamChunks) == 0 {
			return errors.New("materialized record stream chunks are required")
		}
		for _, chunk := range record.StreamChunks {
			if err := requireJSONObject(chunk, "stream chunk"); err != nil {
				return err
			}
		}
		return nil
	}
	return requireJSONObject(record.Response, "response")
}

func replayMaterializedRecord(record MaterializedReplayRecord, stream bool, ctx wrapper.HttpContext, log logs.Log) error {
	ctx.SetUserAttribute("cache_status", "hit")
	ctx.SetUserAttribute("upstream_invoked", false)
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)

	if stream {
		body := materializedStreamBody(record.StreamChunks)
		proxywasm.SendHttpResponseWithDetail(200, "ai-cache.hit", [][2]string{{"content-type", "text/event-stream; charset=utf-8"}}, body, -1)
		return nil
	}

	proxywasm.SendHttpResponseWithDetail(200, "ai-cache.hit", [][2]string{{"content-type", "application/json; charset=utf-8"}}, record.Response, -1)
	return nil
}

func requireJSONObject(raw json.RawMessage, name string) error {
	if len(raw) == 0 {
		return fmt.Errorf("materialized record %s is required", name)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("materialized record %s is not valid JSON object", name)
	}
	if fields == nil {
		return fmt.Errorf("materialized record %s is not a JSON object", name)
	}
	return nil
}

func materializedStreamBody(chunks []json.RawMessage) []byte {
	var body strings.Builder
	for _, chunk := range chunks {
		body.WriteString("data: ")
		body.Write(chunk)
		body.WriteString("\n\n")
	}
	body.WriteString("data: [DONE]\n\n")
	return []byte(body.String())
}
