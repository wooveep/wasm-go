## Context

Provider defaults are merged only for capability keys that were not configured.
This makes an existing configured empty value a natural tombstone, but capability
lookup currently checks key presence alone. The complete Responses-to-Chat fallback
already exists and is selected from that lookup result.

## Goals / Non-Goals

**Goals:**

- Make explicit empty paths disable native capability support.
- Preserve provider defaults and native behavior otherwise.
- Exercise both non-streaming and streaming Qwen fallback behavior.

**Non-Goals:**

- Do not remove Qwen's default native Responses capability.
- Do not add fields to the shared `ProviderConfig` structure.
- Do not implement a second conversion path.

## Decisions

`provider.isSupportedAPI` will check the Qwen Responses case before returning its
existing key-presence result. If the effective capability map explicitly contains
an empty or whitespace-only `openai/v1/responses` path, the lookup reports that API
as unsupported while leaving the configured entry in the map. Omitted and
non-empty Qwen Responses paths retain their existing behavior. The condition is
limited to Qwen Responses, so Gemini, Vertex, and every other provider continue to
treat an empty capability path as supported when the key is present. No CRITICAL
shared default initializer or provider-struct change is needed.

## Risks / Trade-offs

- **[Risk] An accidental empty Qwen Responses value disables that API** → Document
  the Qwen-specific sentinel and add omitted, non-empty, empty, and whitespace tests.
- **[Risk] Fallback is selected without Chat support** → Keep the existing routing
  guard requiring Chat Completions capability.
- **[Trade-off] Empty string gains a Qwen-specific meaning** → The scope avoids
  changing other providers that legitimately use empty paths.

## Migration Plan

Rebuild and upload `ai-proxy`, then add the empty Responses path only to affected
provider configuration. Roll back by removing the override and restoring the prior
artifact.

## Open Questions

None.
