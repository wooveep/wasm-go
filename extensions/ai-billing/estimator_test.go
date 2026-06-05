package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tiktoken-go/tokenizer"
)

func TestTokenizerEncodingForModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  tokenizer.Encoding
	}{
		{name: "empty model falls back to o200k", model: "", want: tokenizer.O200kBase},
		{name: "unknown model falls back to o200k", model: "provider-new-model", want: tokenizer.O200kBase},
		{name: "non openai compatible model falls back to o200k", model: "qwen-plus", want: tokenizer.O200kBase},
		{name: "newer openai model uses o200k", model: "gpt-4o-mini", want: tokenizer.O200kBase},
		{name: "newly matched openai model uses o200k", model: "gpt-4.1", want: tokenizer.O200kBase},
		{name: "older gpt-4 base uses cl100k", model: "gpt-4", want: tokenizer.Cl100kBase},
		{name: "older gpt-4 dated model uses cl100k", model: "gpt-4-0613", want: tokenizer.Cl100kBase},
		{name: "older gpt-3.5 model uses cl100k", model: "gpt-3.5-turbo", want: tokenizer.Cl100kBase},
		{name: "azure gpt-35 alias uses cl100k", model: "gpt-35-turbo-16k", want: tokenizer.Cl100kBase},
		{name: "fine tuned older model uses cl100k", model: "ft:gpt-3.5-turbo:org:suffix", want: tokenizer.Cl100kBase},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tokenizerEncodingForModel(tc.model))
		})
	}
}

func TestTokenizerForModelUsesSelectedEncoding(t *testing.T) {
	codec, err := tokenizerForModel("provider-new-model")
	require.NoError(t, err)
	require.Equal(t, string(tokenizer.O200kBase), codec.GetName())

	count, err := codec.Count("hello")
	require.NoError(t, err)
	require.Greater(t, count, 0)
}

func TestExtractRequestInputTextFromSupportedRequestShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "chat completions messages string content",
			body: `{"model":"gpt-4o-mini","messages":[{"role":"system","content":"answer briefly"},{"role":"user","content":"hello"}]}`,
			want: "answer briefly\nhello",
		},
		{
			name: "chat completions messages content blocks",
			body: `{"messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"https://example.test/image.png"}},{"text":"using two words"}]}]}`,
			want: "describe this\nusing two words",
		},
		{
			name: "chat completions content block type gates text",
			body: `{"messages":[{"role":"user","content":[{"type":"file","text":"do-not-tokenize"},{"type":"text","text":"real prompt"}]}]}`,
			want: "real prompt",
		},
		{
			name: "responses instructions and string input",
			body: `{"instructions":"be concise","input":"summarize the release notes","metadata":{"trace":"do-not-tokenize"}}`,
			want: "be concise\nsummarize the release notes",
		},
		{
			name: "responses input content blocks",
			body: `{"input":[{"role":"user","content":[{"type":"input_text","text":"first question"},{"type":"input_image","image_url":"https://example.test/input.png"}]},{"role":"assistant","content":[{"type":"output_text","text":"prior answer"}]}]}`,
			want: "first question\nprior answer",
		},
		{
			name: "completions prompt string",
			body: `{"prompt":"write a haiku","suffix":"do-not-tokenize"}`,
			want: "write a haiku",
		},
		{
			name: "completions prompt preserves whitespace",
			body: "{\"prompt\":\"  write exactly\\n  \"}",
			want: "  write exactly\n  ",
		},
		{
			name: "completions prompt array",
			body: `{"prompt":["first prompt","second prompt"],"model":"legacy-model"}`,
			want: "first prompt\nsecond prompt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractRequestInputText([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExtractRequestInputTextFailsClosedForUnsupportedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"messages":`},
		{name: "trailing garbage after json", body: `{"prompt":"hi"} trailing`},
		{name: "non object json", body: `["raw text"]`},
		{name: "empty object", body: `{}`},
		{name: "unsupported metadata only", body: `{"model":"gpt-4o-mini","metadata":{"prompt":"do-not-tokenize"}}`},
		{name: "image content without text", body: `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/image.png"}}]}]}`},
		{name: "tool call metadata without content text", body: `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"function":{"name":"lookup","arguments":"{\"city\":\"Paris\"}"}}]}]}`},
		{name: "completion token ids are unsupported", body: `{"prompt":[1,2,3]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractRequestInputText([]byte(tc.body))
			require.False(t, ok)
			require.Empty(t, got)
		})
	}
}
