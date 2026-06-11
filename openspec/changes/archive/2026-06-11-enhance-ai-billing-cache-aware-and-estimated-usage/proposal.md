## Why

`extensions/ai-billing` currently depends primarily on provider-returned usage data, which leaves billing events incomplete when providers omit usage or when streaming clients disconnect before a final usage chunk is observed. The plugin needs a provider-first usage contract with conservative local estimation fallback so billing remains available for text requests without treating estimates as authoritative.

## What Changes

- Extend provider usage parsing to recognize cache-aware usage shapes from OpenAI-compatible/GLM, Kimi, DeepSeek, Qwen, Claude/Anthropic, and Gemini responses.
- Preserve complete raw provider usage under `usage.details.provider_usage` whenever provider usage is available.
- Add provider-first usage source semantics: `provider` when provider usage is used, `estimated` when tokenizer fallback succeeds, and `missing` when no usable usage can be produced.
- Add tokenizer-based basic usage estimation for text-only Chat Completions, Responses, and Completions requests when provider usage is absent.
- Default unknown, empty, or unmapped models to the `o200k_base` tokenizer vocabulary, while allowing recognized older OpenAI-compatible models to use `cl100k_base`.
- Accumulate streamed text deltas sent to the client for output estimation and emit a fallback billing event when the stream finishes without an end-of-stream billing event.
- Add per-request billing delivery idempotency so normal stream completion and stream-done fallback cannot send duplicate events.
- Keep estimated usage limited to basic `input`, `output`, and `total` token counts without cache-aware fields or raw provider usage details.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ai-billing-events`: Strengthen usage parsing, usage source reporting, tokenizer fallback behavior, and streaming completion/interruption billing event guarantees.

## Impact

- Affected code: `extensions/ai-billing` request/response handling, streaming response handling, billing event construction, and usage parsing paths.
- Affected shared logic: `pkg/tokenusage` or adjacent usage extraction helpers if the implementation centralizes provider usage parsing and estimation there.
- Dependencies: add `github.com/tiktoken-go/tokenizer` for local text token estimation.
- Runtime behavior: billing events remain provider-authoritative when provider usage exists, but text requests without usage can now produce estimated basic token counts.
- Event payloads: provider usage events include `usage_source=provider` and raw `usage.details.provider_usage`; estimated events include `usage_source=estimated`; missing events include `usage_source=missing`.
