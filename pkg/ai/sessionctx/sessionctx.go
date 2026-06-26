package sessionctx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode"
)

const (
	defaultTenantHeader    = "x-mse-tenant"
	defaultConsumerHeader  = "x-mse-consumer"
	defaultSessionHeader   = "x-openclaw-session-key"
	defaultRequestIDHeader = "x-request-id"
)

type RequestFactOptions struct {
	TenantHeader    string
	ConsumerHeader  string
	SessionHeader   string
	RequestIDHeader string
}

type RequestFacts struct {
	Tenant    string
	Consumer  string
	SessionID string
	RequestID string
}

func ExtractRequestFacts(headers [][2]string, opts RequestFactOptions, propertyRequestID string) RequestFacts {
	tenantHeader := defaultString(opts.TenantHeader, defaultTenantHeader)
	consumerHeader := defaultString(opts.ConsumerHeader, defaultConsumerHeader)
	sessionHeader := defaultString(opts.SessionHeader, defaultSessionHeader)
	requestIDHeader := defaultString(opts.RequestIDHeader, defaultRequestIDHeader)

	requestID := HeaderValue(headers, requestIDHeader)
	if requestID == "" {
		requestID = propertyRequestID
	}
	return RequestFacts{
		Tenant:    HeaderValue(headers, tenantHeader),
		Consumer:  HeaderValue(headers, consumerHeader),
		SessionID: HeaderValue(headers, sessionHeader),
		RequestID: requestID,
	}
}

func HeaderValue(headers [][2]string, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			return header[1]
		}
	}
	return ""
}

func IsJSONContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func PathMatchesSuffixes(path string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	pathOnly := strings.Split(path, "?")[0]
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(pathOnly, suffix) {
			return true
		}
	}
	return false
}

type OpenAIChatRequest struct {
	Model          string          `json:"model,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	Messages       []OpenAIMessage `json:"messages,omitempty"`
	Tools          json.RawMessage `json:"tools,omitempty"`
	ToolChoice     json.RawMessage `json:"tool_choice,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

type OpenAIMessage struct {
	Role    string      `json:"role,omitempty"`
	Content interface{} `json:"content,omitempty"`
}

func ParseOpenAIChatRequest(body []byte) (OpenAIChatRequest, error) {
	var request OpenAIChatRequest
	err := json.Unmarshal(body, &request)
	return request, err
}

func CurrentUserIntent(messages []OpenAIMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return textContent(messages[i].Content)
	}
	return ""
}

type RequestDigestInput struct {
	Model          string          `json:"model,omitempty"`
	Messages       []OpenAIMessage `json:"messages,omitempty"`
	Tools          json.RawMessage `json:"tools,omitempty"`
	ToolChoice     json.RawMessage `json:"tool_choice,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

func BuildRequestDigest(input RequestDigestInput) (string, error) {
	payload := struct {
		Model          string          `json:"model,omitempty"`
		Messages       []OpenAIMessage `json:"messages,omitempty"`
		Tools          interface{}     `json:"tools,omitempty"`
		ToolChoice     interface{}     `json:"tool_choice,omitempty"`
		ResponseFormat interface{}     `json:"response_format,omitempty"`
	}{
		Model:    input.Model,
		Messages: input.Messages,
	}
	var err error
	if payload.Tools, err = canonicalRawMessage(input.Tools); err != nil {
		return "", err
	}
	if payload.ToolChoice, err = canonicalRawMessage(input.ToolChoice); err != nil {
		return "", err
	}
	if payload.ResponseFormat, err = canonicalRawMessage(input.ResponseFormat); err != nil {
		return "", err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type OpenAIChatResponse struct {
	AssistantContent  string
	FinishReason      string
	Usage             Usage
	ContainsToolCalls bool
}

func ParseOpenAIChatResponse(body []byte) (OpenAIChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content      interface{}   `json:"content"`
				ToolCalls    []interface{} `json:"tool_calls"`
				FunctionCall interface{}   `json:"function_call"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return OpenAIChatResponse{}, err
	}

	response := OpenAIChatResponse{Usage: raw.Usage}
	if len(raw.Choices) == 0 {
		return response, nil
	}
	choice := raw.Choices[0]
	response.AssistantContent = textContent(choice.Message.Content)
	response.FinishReason = choice.FinishReason
	response.ContainsToolCalls = len(choice.Message.ToolCalls) > 0 ||
		choice.Message.FunctionCall != nil ||
		choice.FinishReason == "tool_calls" ||
		choice.FinishReason == "function_call"
	return response, nil
}

type StreamCaptureOptions struct{}

type StreamCapture struct {
	content           strings.Builder
	buffer            string
	finishReason      string
	containsToolCalls bool
}

func NewStreamCapture(StreamCaptureOptions) *StreamCapture {
	return &StreamCapture{}
}

func (c *StreamCapture) AppendSSE(chunk []byte) error {
	c.buffer += string(chunk)
	for {
		index := strings.Index(c.buffer, "\n")
		if index < 0 {
			return nil
		}
		line := strings.TrimSpace(c.buffer[:index])
		c.buffer = c.buffer[index+1:]
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
					Content      interface{}   `json:"content"`
					ToolCalls    []interface{} `json:"tool_calls"`
					FunctionCall interface{}   `json:"function_call"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return err
		}
		for _, choice := range event.Choices {
			c.content.WriteString(textContent(choice.Delta.Content))
			if len(choice.Delta.ToolCalls) > 0 || choice.Delta.FunctionCall != nil {
				c.containsToolCalls = true
			}
			if choice.FinishReason != "" {
				c.finishReason = choice.FinishReason
			}
		}
	}
	return nil
}

func (c *StreamCapture) AssistantContent() string {
	return c.content.String()
}

func (c *StreamCapture) FinishReason() string {
	return c.finishReason
}

func (c *StreamCapture) ContainsToolCalls() bool {
	return c.containsToolCalls
}

type EventEnvelopeInput struct {
	EventKind     string
	Tenant        string
	Consumer      string
	RequestID     string
	RequestDigest string
	StartedAtMS   int64
	EndedAtMS     int64
}

type EventEnvelope struct {
	EventID        string `json:"event_id"`
	IdempotencyKey string `json:"idempotency_key"`
	EventKind      string `json:"event_kind"`
	Tenant         string `json:"tenant,omitempty"`
	Consumer       string `json:"consumer,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	RequestDigest  string `json:"request_digest,omitempty"`
	StartedAtMS    int64  `json:"started_at_ms,omitempty"`
	EndedAtMS      int64  `json:"ended_at_ms,omitempty"`
}

func NewEventEnvelope(input EventEnvelopeInput) EventEnvelope {
	idempotency := stableDigest(struct {
		EventKind     string
		Tenant        string
		Consumer      string
		RequestID     string
		RequestDigest string
	}{
		EventKind:     input.EventKind,
		Tenant:        input.Tenant,
		Consumer:      input.Consumer,
		RequestID:     input.RequestID,
		RequestDigest: input.RequestDigest,
	})
	return EventEnvelope{
		EventID:        stableDigest(input),
		IdempotencyKey: idempotency,
		EventKind:      input.EventKind,
		Tenant:         input.Tenant,
		Consumer:       input.Consumer,
		RequestID:      input.RequestID,
		RequestDigest:  input.RequestDigest,
		StartedAtMS:    input.StartedAtMS,
		EndedAtMS:      input.EndedAtMS,
	}
}

var sensitiveLogKeys = []string{
	"authorization",
	"x-api-key",
	"x-internal-bearer",
	"x-redis-password",
	"x-provider-api-key",
}

func RedactForLog(value string) string {
	result := value
	for _, key := range sensitiveLogKeys {
		result = redactLogKey(result, key)
	}
	return result
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func textContent(value interface{}) string {
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

func stableDigest(value interface{}) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func redactLogKey(value, key string) string {
	var out strings.Builder
	lower := strings.ToLower(value)
	searchStart := 0
	for {
		index := strings.Index(lower[searchStart:], key)
		if index < 0 {
			out.WriteString(value[searchStart:])
			return out.String()
		}
		index += searchStart
		out.WriteString(value[searchStart : index+len(key)])
		cursor := index + len(key)
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			out.WriteByte(value[cursor])
			cursor++
		}
		if cursor < len(value) && value[cursor] == '"' {
			out.WriteByte(value[cursor])
			cursor++
		}
		if cursor >= len(value) || (value[cursor] != '=' && value[cursor] != ':') {
			searchStart = cursor
			continue
		}
		out.WriteByte(value[cursor])
		cursor++
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			out.WriteByte(value[cursor])
			cursor++
		}
		quotedValue := cursor < len(value) && value[cursor] == '"'
		if quotedValue {
			out.WriteByte(value[cursor])
			cursor++
		}
		out.WriteString("[redacted]")
		if quotedValue {
			cursor = quotedLogValueEnd(value, cursor)
			if cursor < len(value) && value[cursor] == '"' {
				out.WriteByte(value[cursor])
				cursor++
			}
		} else {
			cursor = logValueEnd(value, cursor)
		}
		searchStart = cursor
	}
}

func canonicalRawMessage(raw json.RawMessage) (interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func logValueEnd(value string, start int) int {
	cursor := start
	for cursor < len(value) {
		if unicode.IsSpace(rune(value[cursor])) && nextLogTokenHasSeparator(value[cursor+1:]) {
			return cursor
		}
		cursor++
	}
	return cursor
}

func quotedLogValueEnd(value string, start int) int {
	escaped := false
	for cursor := start; cursor < len(value); cursor++ {
		switch {
		case escaped:
			escaped = false
		case value[cursor] == '\\':
			escaped = true
		case value[cursor] == '"':
			return cursor
		}
	}
	return len(value)
}

func nextLogTokenHasSeparator(value string) bool {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return false
	}
	for i, ch := range trimmed {
		if unicode.IsSpace(ch) {
			return false
		}
		if ch == '=' || ch == ':' {
			return i > 0
		}
	}
	return false
}
