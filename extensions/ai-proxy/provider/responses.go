package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
)

type responsesRequest struct {
	Model              string            `json:"model"`
	Instructions       string            `json:"instructions,omitempty"`
	Input              json.RawMessage   `json:"input"`
	Stream             bool              `json:"stream,omitempty"`
	Temperature        *float64          `json:"temperature,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	PresencePenalty    *float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty   *float64          `json:"frequency_penalty,omitempty"`
	MaxOutputTokens    int               `json:"max_output_tokens,omitempty"`
	User               string            `json:"user,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	Tools              []responsesTool   `json:"tools,omitempty"`
	ToolChoice         json.RawMessage   `json:"tool_choice,omitempty"`
	PreviousResponseID string            `json:"previous_response_id,omitempty"`
	Conversation       any               `json:"conversation,omitempty"`
}

type responsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type responsesMessageInput struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type responsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func convertResponsesRequestToChatCompletion(body []byte) ([]byte, error) {
	if gjson.GetBytes(body, "previous_response_id").Exists() {
		return nil, errors.New("previous_response_id is unsupported in Responses fallback mode")
	}
	if gjson.GetBytes(body, "conversation").Exists() {
		return nil, errors.New("conversation is unsupported in Responses fallback mode")
	}

	var request responsesRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("unable to unmarshal responses request: %w", err)
	}

	messages, err := convertResponsesInputToChatMessages(request.Input)
	if err != nil {
		return nil, err
	}
	if request.Instructions != "" {
		messages = append([]chatMessage{{
			Role:    roleDeveloper,
			Content: request.Instructions,
		}}, messages...)
	}

	chatRequest := chatCompletionRequest{
		Model:      request.Model,
		Messages:   messages,
		Stream:     request.Stream,
		User:       request.User,
		Metadata:   request.Metadata,
		ToolChoice: nil,
		Tools:      nil,
	}
	if request.Stream {
		chatRequest.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	if request.Temperature != nil {
		chatRequest.Temperature = *request.Temperature
	}
	if request.TopP != nil {
		chatRequest.TopP = *request.TopP
	}
	if request.PresencePenalty != nil {
		chatRequest.PresencePenalty = *request.PresencePenalty
	}
	if request.FrequencyPenalty != nil {
		chatRequest.FrequencyPenalty = *request.FrequencyPenalty
	}
	if request.MaxOutputTokens > 0 {
		chatRequest.MaxCompletionTokens = request.MaxOutputTokens
	}
	tools, err := convertResponsesTools(request.Tools)
	if err != nil {
		return nil, err
	}
	chatRequest.Tools = tools
	toolChoice, err := convertResponsesToolChoice(request.ToolChoice)
	if err != nil {
		return nil, err
	}
	chatRequest.ToolChoice = toolChoice
	return json.Marshal(chatRequest)
}

func convertResponsesInputToChatMessages(input json.RawMessage) ([]chatMessage, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, errors.New("input is required for Responses fallback mode")
	}
	var text string
	if err := json.Unmarshal(input, &text); err == nil {
		return []chatMessage{{Role: roleUser, Content: text}}, nil
	}

	var items []responsesMessageInput
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("unsupported responses input: %w", err)
	}
	messages := make([]chatMessage, 0, len(items))
	for _, item := range items {
		content, err := convertResponsesMessageContent(item.Content)
		if err != nil {
			return nil, err
		}
		messages = append(messages, chatMessage{
			Role:    item.Role,
			Content: content,
		})
	}
	return messages, nil
}

func convertResponsesMessageContent(content json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text, nil
	}

	var parts []responsesContentPart
	if err := json.Unmarshal(content, &parts); err != nil {
		return "", fmt.Errorf("unsupported responses message content: %w", err)
	}
	var builder bytes.Buffer
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text":
			builder.WriteString(part.Text)
		default:
			return "", fmt.Errorf("%s is unsupported in Responses fallback mode", part.Type)
		}
	}
	return builder.String(), nil
}

func convertResponsesTools(tools []responsesTool) ([]tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	chatTools := make([]tool, 0, len(tools))
	for _, responsesTool := range tools {
		if responsesTool.Type != "function" {
			return nil, fmt.Errorf("%s is unsupported in Responses fallback mode", responsesTool.Type)
		}
		chatTools = append(chatTools, tool{
			Type: "function",
			Function: function{
				Name:        responsesTool.Name,
				Description: responsesTool.Description,
				Parameters:  responsesTool.Parameters,
			},
		})
	}
	return chatTools, nil
}

func convertResponsesToolChoice(raw json.RawMessage) (interface{}, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var choiceString string
	if err := json.Unmarshal(raw, &choiceString); err == nil {
		switch choiceString {
		case "auto", "none", "required":
			return choiceString, nil
		default:
			return nil, fmt.Errorf("tool_choice %q is unsupported in Responses fallback mode", choiceString)
		}
	}

	var choice struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("unsupported tool_choice in Responses fallback mode: %w", err)
	}
	switch choice.Type {
	case "auto", "none", "required":
		return choice.Type, nil
	case "function":
		if choice.Name == "" {
			return nil, errors.New("function tool_choice requires name in Responses fallback mode")
		}
		return &toolChoice{
			Type: "function",
			Function: function{
				Name: choice.Name,
			},
		}, nil
	default:
		return nil, fmt.Errorf("tool_choice type %q is unsupported in Responses fallback mode", choice.Type)
	}
}

func convertChatCompletionResponseToResponses(body []byte) ([]byte, error) {
	return nil, errors.New("chat completions to responses conversion is not implemented")
}

type chatCompletionToResponsesStreamConverter struct{}

func newChatCompletionToResponsesStreamConverter() *chatCompletionToResponsesStreamConverter {
	return &chatCompletionToResponsesStreamConverter{}
}

func (c *chatCompletionToResponsesStreamConverter) convert(chunk []byte, isLastChunk bool) ([]byte, error) {
	return nil, errors.New("chat completions stream to responses conversion is not implemented")
}
