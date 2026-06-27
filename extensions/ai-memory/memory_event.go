package main

const (
	memoryEventSchemaVersion = 1
	memoryEventPluginVersion = "0.1.0"
)

type MemoryEvent struct {
	SchemaVersion int `json:"schema_version"`

	MemoryEventEnvelope
	MemoryEventIdentity
	MemoryEventRequest

	Route *MemoryEventRoute `json:"route,omitempty"`
	Model *MemoryEventModel `json:"model,omitempty"`
	Usage *MemoryEventUsage `json:"usage,omitempty"`

	MemoryEventPolicy
	MemoryEventStatus
	MemoryEventStream
	MemoryEventToolCall
	MemoryEventFinish
	MemoryEventTiming

	PluginVersion string `json:"plugin_version"`
	NoStore       bool   `json:"no_store"`

	UserContent      string `json:"user_content,omitempty"`
	AssistantContent string `json:"assistant_content,omitempty"`
}

type MemoryEventEnvelope struct {
	EventID        string `json:"event_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type MemoryEventIdentity struct {
	Tenant    string `json:"tenant"`
	Consumer  string `json:"consumer"`
	SessionID string `json:"session_id,omitempty"`
}

type MemoryEventRequest struct {
	RequestID     string `json:"request_id"`
	RequestPath   string `json:"request_path"`
	RequestDigest string `json:"request_digest"`
}

type MemoryEventRoute struct {
	Name string `json:"name"`
}

type MemoryEventModel struct {
	Name string `json:"name"`
}

type MemoryEventPolicy struct {
	PolicyVersion string `json:"policy_version,omitempty"`
	MemoryMode    string `json:"memory_mode"`
}

type MemoryEventStatus struct {
	StatusCode int `json:"status_code"`
}

type MemoryEventStream struct {
	IsStream bool `json:"is_stream"`
}

type MemoryEventToolCall struct {
	ContainsToolCalls bool `json:"contains_tool_calls"`
}

type MemoryEventUsage struct {
	Unit   string `json:"unit"`
	Input  int    `json:"input"`
	Output int    `json:"output"`
	Total  int    `json:"total"`
}

type MemoryEventFinish struct {
	FinishReason string `json:"finish_reason,omitempty"`
}

type MemoryEventTiming struct {
	StartedAtMS int64 `json:"started_at_ms"`
	EndedAtMS   int64 `json:"ended_at_ms"`
}
