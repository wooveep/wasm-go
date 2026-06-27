package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryEventStructJSONShape(t *testing.T) {
	t.Run("marshals flat required facts with nested route model and normalized usage", func(t *testing.T) {
		event := MemoryEvent{
			SchemaVersion:       memoryEventSchemaVersion,
			MemoryEventEnvelope: MemoryEventEnvelope{EventID: "event-1", IdempotencyKey: "idem-1"},
			MemoryEventIdentity: MemoryEventIdentity{Tenant: "tenant-a", Consumer: "consumer-a", SessionID: "session-a"},
			MemoryEventRequest:  MemoryEventRequest{RequestID: "request-1", RequestPath: "/v1/chat/completions", RequestDigest: "sha256:abc"},
			Route:               &MemoryEventRoute{Name: "memory-route"},
			Model:               &MemoryEventModel{Name: "qwen-turbo"},
			MemoryEventPolicy:   MemoryEventPolicy{PolicyVersion: "policy-v1", MemoryMode: "semantic"},
			MemoryEventStatus:   MemoryEventStatus{StatusCode: 200},
			MemoryEventStream:   MemoryEventStream{IsStream: true},
			MemoryEventToolCall: MemoryEventToolCall{ContainsToolCalls: true},
			Usage:               &MemoryEventUsage{Unit: "token", Input: 12, Output: 5, Total: 17},
			MemoryEventFinish:   MemoryEventFinish{FinishReason: "stop"},
			MemoryEventTiming:   MemoryEventTiming{StartedAtMS: 1782420000000, EndedAtMS: 1782420000500},
			PluginVersion:       memoryEventPluginVersion,
			UserContent:         "safe user",
			AssistantContent:    "safe assistant",
		}

		body, err := json.Marshal(event)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		for _, field := range []string{
			"schema_version",
			"event_id",
			"idempotency_key",
			"tenant",
			"consumer",
			"session_id",
			"request_id",
			"request_path",
			"request_digest",
			"route",
			"model",
			"policy_version",
			"memory_mode",
			"status_code",
			"is_stream",
			"contains_tool_calls",
			"usage",
			"finish_reason",
			"started_at_ms",
			"ended_at_ms",
			"plugin_version",
			"user_content",
			"assistant_content",
		} {
			require.Containsf(t, payload, field, "missing MemoryEvent field %q; payload=%s", field, string(body))
		}
		require.EqualValues(t, 1, payload["schema_version"])
		require.Equal(t, map[string]interface{}{"name": "memory-route"}, payload["route"])
		require.Equal(t, map[string]interface{}{"name": "qwen-turbo"}, payload["model"])
		require.Equal(t, map[string]interface{}{
			"unit":   "token",
			"input":  float64(12),
			"output": float64(5),
			"total":  float64(17),
		}, payload["usage"])
		require.Equal(t, "0.1.0", payload["plugin_version"])
	})

	t.Run("omits optional empty fields", func(t *testing.T) {
		event := MemoryEvent{
			SchemaVersion:       memoryEventSchemaVersion,
			MemoryEventEnvelope: MemoryEventEnvelope{EventID: "event-2", IdempotencyKey: "idem-2"},
			MemoryEventIdentity: MemoryEventIdentity{Tenant: "tenant-a", Consumer: "consumer-a"},
			MemoryEventRequest:  MemoryEventRequest{RequestID: "request-2", RequestPath: "/v1/chat/completions", RequestDigest: "sha256:def"},
			MemoryEventPolicy:   MemoryEventPolicy{MemoryMode: "recent-only"},
			MemoryEventStatus:   MemoryEventStatus{StatusCode: 204},
			MemoryEventToolCall: MemoryEventToolCall{},
			MemoryEventTiming:   MemoryEventTiming{StartedAtMS: 1782420000000, EndedAtMS: 1782420000500},
			PluginVersion:       memoryEventPluginVersion,
		}

		body, err := json.Marshal(event)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		for _, field := range []string{
			"session_id",
			"route",
			"model",
			"policy_version",
			"usage",
			"finish_reason",
			"user_content",
			"assistant_content",
		} {
			require.NotContainsf(t, payload, field, "expected empty optional field %q to be omitted; payload=%s", field, string(body))
		}
		require.Contains(t, payload, "no_store")
		require.Equal(t, false, payload["no_store"])
		require.Contains(t, payload, "contains_tool_calls")
		require.Equal(t, false, payload["contains_tool_calls"])
		require.Contains(t, payload, "is_stream")
		require.Equal(t, false, payload["is_stream"])
	})
}
