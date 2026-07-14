## Why

Qwen compatible mode advertises a default native Responses endpoint even for
models that only work through its compatible Chat Completions endpoint, so
multi-provider `/v1/responses` calls can fail before the existing fallback runs.

## What Changes

- Treat an explicitly empty or whitespace-only Qwen Responses capability path as a
  native API opt-out while preserving Gemini, Vertex, and other provider
  empty-path behavior.
- Reuse the existing Responses-to-Chat request, response, and SSE conversion when
  Chat Completions remains available.
- Document and test the capability sentinel, including Qwen compatible mode.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `openai-responses-api`: Refine Qwen native capability lookup so an explicit
  empty or whitespace-only Responses path selects the existing compatible fallback
  when possible, without changing other providers' empty-path semantics.

## Impact

- `extensions/ai-proxy/provider` Qwen Responses support lookup.
- Qwen and Responses fallback tests and bilingual documentation.
- No shared provider type/schema expansion and no change to provider defaults.
