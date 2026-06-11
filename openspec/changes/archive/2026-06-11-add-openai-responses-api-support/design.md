## Context

ai-proxy already has the `ApiNameResponses` and `PathOpenAIResponses` identifiers, suffix-based detection for `/v1/responses`, and default OpenAI provider capability mapping for `openai/v1/responses`. Some provider-specific paths, such as compatible-mode providers, can already pass Responses traffic through natively.

The missing behavior is the generic automatic protocol fallback that `/v1/messages` already has: when the client sends a protocol the selected provider does not support natively, ai-proxy should convert the request into the internal OpenAI Chat Completions text generation protocol, reuse the existing provider conversion chain, and convert the provider response back to the client protocol. For Responses this must cover both buffered JSON responses and SSE streaming responses.

## Goals / Non-Goals

**Goals:**

- Treat `/v1/responses` as an OpenAI Responses API request for routing, capability selection, request body processing, response body processing, and streaming.
- Prefer native provider Responses support when `openai/v1/responses` exists in the active provider capability map.
- Fall back to Chat Completions only when the provider lacks Responses support and supports `openai/v1/chatcompletions`.
- Keep the client-facing protocol as Responses in fallback mode.
- Preserve existing Chat Completions, Anthropic Messages, and provider-native text generation behavior.
- Document field mappings, fallback limitations, and provider capability configuration.

**Non-Goals:**

- Do not implement server-side Responses state storage in ai-proxy.
- Do not emulate `previous_response_id`, `conversation`, retrieve, delete, cancel, compact, or input item listing in fallback mode.
- Do not convert hosted tools such as web search, file search, computer use, code interpreter, MCP tools, or custom tools beyond the safe function-tool subset.
- Do not add a new provider conversion chain; fallback mode should reuse the existing Chat Completions chain.

## Decisions

### Native capability wins

When the detected API name is `ApiNameResponses`, the header phase should first check `ProviderConfig.IsSupportedAPI(ApiNameResponses)`. If true, ai-proxy keeps the API name and original client protocol. Provider header transformation then maps the path with `openai/v1/responses` to the provider's configured Responses endpoint, and no response conversion marker is set.

Alternative considered: always normalize Responses into Chat Completions. That would lose native Responses features such as state, hosted tools, and provider-specific streaming event shapes for providers that support them.

### Fallback uses Chat Completions as the intermediate protocol

If the provider lacks Responses support but supports `ApiNameChatCompletion`, the header phase should rewrite the path suffix from `/v1/responses` to `/v1/chat/completions`, change the internal `apiName` to `ApiNameChatCompletion`, and set a context marker such as `needResponsesResponseConversion`. The request body phase should convert the original Responses body to Chat Completions before the provider-specific `TransformRequestBody` or `TransformRequestBodyHeaders` hook runs.

This mirrors the current `/v1/messages` auto-detection pattern and keeps model mapping, provider base path handling, context cleanup, developer-role normalization, and existing vendor converters in one path.

Alternative considered: add Responses handling to every provider. That would duplicate text generation conversion logic and create inconsistent support across providers.

### Request conversion is explicit and conservative

The fallback converter should construct a Chat Completions request from the Responses request:

- `model` maps to `model`; existing model mapping still runs after conversion.
- `input` string maps to one user message.
- `input` message/item arrays map supported message items to Chat Completions `messages`.
- `instructions` maps to a leading developer message so existing provider role normalization can downgrade it to system where needed.
- `stream`, `temperature`, `top_p`, `presence_penalty`, `frequency_penalty`, `user`, `metadata`, and compatible generation controls map to equivalent Chat Completions fields when available.
- `max_output_tokens` maps to `max_completion_tokens`.
- `tools` only maps `type: "function"` tools to Chat Completions function tools; unsupported hosted or custom tool types return a clear unsupported error in fallback mode.
- `tool_choice` maps only safe Chat Completions-compatible choices such as `auto`, `none`, `required`, and function choices.

Fields with no safe fallback semantics should not be silently forwarded. Stateful fields and hosted tools should return unsupported; harmless metadata-like fields may be dropped with debug logging and documented.

### Response conversion happens after provider conversion

In fallback mode, providers should still return OpenAI Chat Completions shaped data after their existing response transformer runs. ai-proxy should then convert that Chat Completions JSON to a Responses response object:

- `choices[0].message.content` becomes an assistant message output item with `content[].type = "output_text"`.
- `choices[0].message.refusal` becomes Responses refusal content.
- `usage.prompt_tokens`, `usage.completion_tokens`, and `usage.total_tokens` become Responses `usage.input_tokens`, `usage.output_tokens`, and `usage.total_tokens`.
- `id`, `created`, `model`, finish reason, status, and errors should be preserved where the Responses schema has corresponding fields.

Native Responses providers must bypass this conversion because their response already uses the client protocol.

### Streaming conversion wraps Chat Completions SSE as Responses SSE

In fallback mode, the streaming path should convert Chat Completions chunks after provider streaming transformation. Text deltas from `chat.completion.chunk` should become `response.output_text.delta` events. Finish or `[DONE]` must not be exposed as Chat Completions data; it should finalize the Responses stream with completion events such as `response.output_text.done` and `response.completed`.

The converter should keep minimal stream state in the request context, including response id, output item id, accumulated text for final done events, finish reason, and usage when present. Native Responses SSE should be returned unchanged.

### Unsupported behavior is visible

If the provider supports neither Responses nor Chat Completions, ai-proxy should return or record an explicit unsupported API error instead of letting `/v1/responses` continue to an unrelated provider path. In fallback mode, `previous_response_id`, `conversation`, and stateful subresource paths must be unsupported because ai-proxy does not store Responses state.

## Risks / Trade-offs

- [Risk] Responses has a larger schema than Chat Completions, so fallback mode cannot preserve every field. -> Mitigation: support a documented safe subset and fail explicitly for fields that would change semantics if ignored.
- [Risk] Streaming event ordering in the Responses API is richer than the minimal text-delta path. -> Mitigation: implement deterministic text streaming events first, maintain context state, and test `[DONE]` handling so Chat Completions chunks never leak to the client.
- [Risk] Some providers emit non-standard Chat Completions fields. -> Mitigation: convert only stable fields and preserve unknown provider-specific behavior inside native Responses mode rather than fallback mode.
- [Risk] Existing `/v1/messages` auto-conversion and Chat Completions behavior could regress. -> Mitigation: add regression tests for `/v1/messages`, `/v1/chat/completions`, native Responses, and fallback Responses routing.

## Migration Plan

No operator migration is required for existing routes. Providers that support Responses natively can add `openai/v1/responses` to `capabilities` with the desired provider path. Providers without that capability but with `openai/v1/chatcompletions` will use fallback conversion.

Rollback is code-level: removing the fallback conversion should return `/v1/responses` to the existing native-only behavior. Documentation should make clear that stateful Responses features require native provider support.

## Open Questions

- Which HTTP status and error body helper should be used for fallback-mode unsupported fields if the current plugin only records some provider processing errors through `util.ErrorHandler`?
- Should fallback mode preserve `store` as `false` only, or reject `store: true` because ai-proxy does not implement Responses state?
