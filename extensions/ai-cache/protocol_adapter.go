package main

import (
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

type thinProtocolAdapter interface {
	BuildCacheDigest(protocol.RequestParseInput) (protocol.CacheDigestResult, error)
	CaptureResponse(protocol.ResponseCaptureInput) (protocol.NormalizedExchange, error)
	CaptureStream(protocol.ResponseStreamInput) (protocol.NormalizedExchange, error)
	SerializeReplay(protocol.ReplayPayload) (protocol.SerializedReplay, error)
}

func thinProtocolAdapterForPath(path string) (protocol.ProtocolKind, thinProtocolAdapter, bool) {
	kind := protocol.DetectProtocolKind(path)
	switch kind {
	case protocol.ProtocolChatCompletions:
		return kind, protocol.ChatCompletionsAdapter{}, true
	case protocol.ProtocolMessages:
		return kind, protocol.MessagesAdapter{}, true
	case protocol.ProtocolResponses:
		return kind, protocol.ResponsesAdapter{}, true
	default:
		return protocol.ProtocolUnknown, nil, false
	}
}

func thinEventUsage(kind protocol.ProtocolKind, usage protocol.Usage) json.RawMessage {
	if usage == (protocol.Usage{}) {
		return nil
	}

	fields := map[string]int{}
	switch kind {
	case protocol.ProtocolChatCompletions:
		addPositiveIntField(fields, "prompt_tokens", usage.InputTokens)
		addPositiveIntField(fields, "completion_tokens", usage.OutputTokens)
	default:
		addPositiveIntField(fields, "input_tokens", usage.InputTokens)
		addPositiveIntField(fields, "output_tokens", usage.OutputTokens)
	}
	addPositiveIntField(fields, "input_cache_hit_tokens", usage.InputCacheHitTokens)
	addPositiveIntField(fields, "input_cache_miss_tokens", usage.InputCacheMissTokens)
	addPositiveIntField(fields, "total_tokens", usage.TotalTokens)
	if len(fields) == 0 {
		return nil
	}
	body, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return body
}

func addPositiveIntField(fields map[string]int, name string, value int) {
	if value > 0 {
		fields[name] = value
	}
}

func thinResponseFactsFromExchange(kind protocol.ProtocolKind, exchange protocol.NormalizedExchange) thinResponseFacts {
	return thinResponseFacts{
		assistantContent:  exchange.Response.Text,
		finishReason:      exchange.Response.FinishReason,
		usage:             thinEventUsage(kind, exchange.Usage),
		containsToolCalls: exchange.Response.ContainsToolCalls,
		parseFailed:       exchange.Response.ParseFailed,
	}
}

func thinReplayPayload(kind protocol.ProtocolKind, record MaterializedReplayRecord, stream bool) protocol.ReplayPayload {
	payload := protocol.ReplayPayload{
		Protocol:   kind,
		StatusCode: 200,
	}
	if stream {
		payload.ContentType = "text/event-stream; charset=utf-8"
		payload.StreamChunks = thinReplayStreamChunks(kind, record.StreamChunks)
		return payload
	}
	payload.ContentType = "application/json; charset=utf-8"
	payload.Body = append(json.RawMessage(nil), record.Response...)
	return payload
}

func thinReplayStreamChunks(kind protocol.ProtocolKind, chunks []json.RawMessage) []protocol.StreamChunk {
	out := make([]protocol.StreamChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, protocol.StreamChunk{
			Protocol:   kind,
			Event:      thinReplayStreamEvent(kind, chunk),
			Data:       append(json.RawMessage(nil), chunk...),
			Replayable: true,
		})
	}
	return out
}

func thinReplayStreamEvent(kind protocol.ProtocolKind, raw json.RawMessage) string {
	if kind == protocol.ProtocolChatCompletions {
		return ""
	}
	var event struct {
		Event string `json:"event"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return ""
	}
	if strings.TrimSpace(event.Event) != "" {
		return event.Event
	}
	return event.Type
}

func thinCurrentUserPromptForPath(path string, body []byte) (string, error) {
	kind := protocol.DetectProtocolKind(path)
	switch kind {
	case protocol.ProtocolChatCompletions, protocol.ProtocolMessages:
		fields, err := decodeJSONObject(body)
		if err != nil {
			return "", err
		}
		return thinCurrentUserPromptFromMessages(fields["messages"])
	case protocol.ProtocolResponses:
		fields, err := decodeJSONObject(body)
		if err != nil {
			return "", err
		}
		return thinCurrentUserPromptFromResponsesInput(fields["input"])
	default:
		return "", nil
	}
}

func thinCurrentUserPromptFromMessages(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &messages); err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return thinTextFromRawContent(messages[i].Content, "text"), nil
	}
	return "", nil
}

func thinCurrentUserPromptFromResponsesInput(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var inputText string
	if err := json.Unmarshal(raw, &inputText); err == nil {
		return inputText, nil
	}
	var items []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", err
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Role != "" && items[i].Role != "user" {
			continue
		}
		if text := thinTextFromRawContent(items[i].Content, "input_text"); text != "" {
			return text, nil
		}
	}
	return "", nil
}

func thinTextFromRawContent(raw json.RawMessage, textType string) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type != textType {
			continue
		}
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "")
}
