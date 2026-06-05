## Why

OpenAI's Responses API is now the primary text generation interface for many clients, but ai-proxy currently only treats OpenAI Chat Completions as the stable text generation protocol. Supporting `/v1/responses` lets clients use the Responses API through ai-proxy while preserving existing provider conversion behavior for providers that only support Chat Completions or vendor-native text generation.

## What Changes

- Recognize request paths whose suffix matches `/v1/responses` as `ApiNameResponses`.
- Add provider capability handling for `openai/v1/responses`, using the configured provider Responses path without rewriting when the provider supports it.
- Add fallback routing for providers that do not support Responses but do support Chat Completions: convert the Responses request to an OpenAI Chat Completions request, rewrite the internal path to `/v1/chat/completions`, and reuse the existing provider text generation conversion chain.
- Add response conversion so fallback mode still returns OpenAI Responses response objects or Responses SSE events to the client.
- Return a clear unsupported API error when a provider supports neither Responses nor Chat Completions text generation.
- Document supported fallback fields, unsupported stateful Responses features, streaming behavior, provider capability configuration, and the native-vs-conversion priority.
- Preserve existing `/v1/messages` and `/v1/chat/completions` behavior.

## Capabilities

### New Capabilities
- `openai-responses-api`: Defines ai-proxy support for OpenAI Responses text generation requests, including provider capability selection, Responses-to-Chat-Completions fallback conversion, response conversion, streaming conversion, unsupported feature handling, and documentation requirements.

### Modified Capabilities
- None.

## Impact

- Affected extension code: `extensions/ai-proxy` request API detection, provider capability selection, path rewrite logic, request body conversion, response body conversion, SSE transformation, context markers, and tests.
- Affected provider configuration: provider capabilities may include `openai/v1/responses` with a configured Responses endpoint path.
- Affected documentation: `extensions/ai-proxy/README.md`, `extensions/ai-proxy/README_EN.md`, and provider capability examples.
- No external dependencies are expected.
- No breaking changes are intended for existing OpenAI Chat Completions, Anthropic Messages, or provider-native text generation flows.
