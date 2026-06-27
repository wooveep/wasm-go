package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	memoryAssembleSchemaVersion = 1

	memoryAssembleDecisionInject     = "inject"
	memoryAssembleDecisionRecentOnly = "recent_only"
	memoryAssembleDecisionSkip       = "skip"
	memoryAssembleDecisionBypass     = "bypass"
)

type memoryNamedFact struct {
	Name string `json:"name"`
}

type memoryAssembleRequest struct {
	SchemaVersion     int             `json:"schema_version"`
	Tenant            string          `json:"tenant"`
	Consumer          string          `json:"consumer"`
	SessionID         string          `json:"session_id,omitempty"`
	Route             memoryNamedFact `json:"route"`
	Model             memoryNamedFact `json:"model"`
	RequestID         string          `json:"request_id"`
	RequestPath       string          `json:"request_path"`
	CurrentQuestion   string          `json:"current_question"`
	MessagesDigest    string          `json:"messages_digest"`
	MemoryMode        string          `json:"memory_mode"`
	RecentWindowTurns int             `json:"recent_window_turns,omitempty"`
	MemoryTokenBudget int             `json:"memory_token_budget,omitempty"`
	SemanticTopK      int             `json:"semantic_top_k,omitempty"`
	PolicyVersion     string          `json:"policy_version,omitempty"`
}

type memoryConsoleAssembleResponse struct {
	SchemaVersion  int                     `json:"schema_version"`
	Decision       string                  `json:"decision"`
	MemoryMessage  *memoryAssembleMessage  `json:"memory_message,omitempty"`
	RecentMessages []memoryAssembleMessage `json:"recent_messages,omitempty"`
	Trace          map[string]interface{}  `json:"trace,omitempty"`
	Diagnostics    map[string]interface{}  `json:"diagnostics,omitempty"`
}

type memoryAssembleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func buildMemoryAssembleRequestBody(ctx wrapper.HttpContext, c config.PluginConfig) ([]byte, error) {
	return json.Marshal(memoryAssembleRequest{
		SchemaVersion:     memoryAssembleSchemaVersion,
		Tenant:            ctx.GetStringContext(memoryTenantContextKey, ""),
		Consumer:          ctx.GetStringContext(memoryConsumerContextKey, ""),
		SessionID:         ctx.GetStringContext(memorySessionContextKey, ""),
		Route:             memoryNamedFact{Name: ctx.GetStringContext(memoryRouteContextKey, "")},
		Model:             memoryNamedFact{Name: ctx.GetStringContext(memoryModelContextKey, "")},
		RequestID:         ctx.GetStringContext(memoryRequestIDContextKey, ""),
		RequestPath:       ctx.GetStringContext(memoryRequestPathContextKey, ""),
		CurrentQuestion:   ctx.GetStringContext(memoryUserContentContextKey, ""),
		MessagesDigest:    ctx.GetStringContext(memoryRequestDigestContextKey, ""),
		MemoryMode:        c.Route.MemoryMode,
		RecentWindowTurns: c.Route.RecentWindowTurns,
		MemoryTokenBudget: c.Route.MemoryTokenBudget,
		SemanticTopK:      c.Route.SemanticTopK,
		PolicyVersion:     c.Route.PolicyVersion,
	})
}

func parseMemoryAssembleResponse(body []byte) (memoryConsoleAssembleResponse, error) {
	var response memoryConsoleAssembleResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return memoryConsoleAssembleResponse{}, errors.New("Console assemble response is not valid JSON")
	}
	if response.SchemaVersion != memoryAssembleSchemaVersion {
		return memoryConsoleAssembleResponse{}, errors.New("unsupported Console assemble schema version")
	}
	if !validMemoryAssembleDecision(response.Decision) {
		return memoryConsoleAssembleResponse{}, fmt.Errorf("unsupported Console assemble decision %q", response.Decision)
	}
	if response.MemoryMessage != nil {
		if err := validateMemoryMessage(*response.MemoryMessage, true); err != nil {
			return memoryConsoleAssembleResponse{}, err
		}
	}
	for _, message := range response.RecentMessages {
		if err := validateMemoryMessage(message, false); err != nil {
			return memoryConsoleAssembleResponse{}, err
		}
	}
	return response, nil
}

func validMemoryAssembleDecision(decision string) bool {
	switch decision {
	case memoryAssembleDecisionInject, memoryAssembleDecisionRecentOnly, memoryAssembleDecisionSkip, memoryAssembleDecisionBypass:
		return true
	default:
		return false
	}
}

func validateMemoryMessage(message memoryAssembleMessage, allowSystem bool) error {
	switch message.Role {
	case "user", "assistant":
	case "system":
		if !allowSystem {
			return errors.New("system role is not supported in recent messages")
		}
	case "developer":
		if !allowSystem {
			return errors.New("developer role is not supported in recent messages")
		}
	default:
		return fmt.Errorf("unsupported memory message role %q", message.Role)
	}
	if len(message.Content) > maxRecentMessageBytes {
		return errors.New("memory message too large")
	}
	return nil
}

func (m memoryAssembleMessage) toOpenAIMessage() sessionctx.OpenAIMessage {
	return sessionctx.OpenAIMessage{
		Role:    m.Role,
		Content: m.Content,
	}
}

func memoryAssembleMessagesToOpenAI(messages []memoryAssembleMessage) []sessionctx.OpenAIMessage {
	out := make([]sessionctx.OpenAIMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.toOpenAIMessage())
	}
	return out
}
