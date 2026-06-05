package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

func tokenizerForModel(model string) (tokenizer.Codec, error) {
	return tokenizer.Get(tokenizerEncodingForModel(model))
}

func tokenizerEncodingForModel(model string) tokenizer.Encoding {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if isCl100kBaseModel(normalizedModel) {
		return tokenizer.Cl100kBase
	}
	return tokenizer.O200kBase
}

func isCl100kBaseModel(model string) bool {
	for _, family := range []string{
		"gpt-4",
		"gpt-3.5",
		"gpt-35",
		"ft:gpt-4",
		"ft:gpt-3.5",
		"ft:davinci-002",
		"ft:babbage-002",
		"text-embedding-ada-002",
		"davinci-002",
		"babbage-002",
	} {
		if matchesModelFamily(model, family) {
			return true
		}
	}
	return false
}

func matchesModelFamily(model, family string) bool {
	return model == family || strings.HasPrefix(model, family+"-") || strings.HasPrefix(model, family+":")
}

func extractRequestInputText(body []byte) (string, bool) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", false
	}

	object, ok := payload.(map[string]any)
	if !ok {
		return "", false
	}

	parts := make([]string, 0)
	appendTextValue(&parts, object["instructions"])
	appendChatMessagesText(&parts, object["messages"])
	appendTextValue(&parts, object["input"])
	appendTextValue(&parts, object["prompt"])

	return joinTextParts(parts)
}

func appendChatMessagesText(parts *[]string, value any) {
	messages, ok := value.([]any)
	if !ok {
		return
	}
	for _, message := range messages {
		messageObject, ok := message.(map[string]any)
		if !ok {
			continue
		}
		appendTextValue(parts, messageObject["content"])
	}
}

func appendTextValue(parts *[]string, value any) {
	switch typed := value.(type) {
	case string:
		*parts = append(*parts, typed)
	case []any:
		for _, item := range typed {
			appendTextValue(parts, item)
		}
	case map[string]any:
		if isTextContentBlock(typed) {
			appendTextValue(parts, typed["text"])
		}
		appendTextValue(parts, typed["content"])
	}
}

func isTextContentBlock(object map[string]any) bool {
	value, hasType := object["type"]
	if !hasType {
		return true
	}
	blockType, ok := value.(string)
	if !ok {
		return false
	}
	switch blockType {
	case "text", "input_text", "output_text":
		return true
	default:
		return false
	}
}

func joinTextParts(parts []string) (string, bool) {
	joinedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			joinedParts = append(joinedParts, part)
		}
	}
	if len(joinedParts) == 0 {
		return "", false
	}
	return strings.Join(joinedParts, "\n"), true
}
