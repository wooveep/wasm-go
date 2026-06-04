package provider

import (
	"encoding/json"
	"errors"
	"fmt"
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
	var request responsesRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("unable to unmarshal responses request: %w", err)
	}

	chatRequest := chatCompletionRequest{
		Model: request.Model,
	}
	return json.Marshal(chatRequest)
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
