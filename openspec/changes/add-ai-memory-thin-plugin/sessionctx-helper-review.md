# sessionctx Helper Review

Date: 2026-06-27

This review keeps `pkg/ai/sessionctx` scoped to policy-free OpenAI protocol,
SSE, Redis Stream command, event envelope, and safe logging helpers. The
`ai-memory` plugin should own memory policy, Console assemble behavior, Redis
recent-memory rules, raw-content gates, and MemoryEvent schema composition.

## Existing Helpers To Reuse

- Request facts: `ExtractRequestFacts`, `HeaderValue`, `IsJSONContentType`, and
  `PathMatchesSuffixes` cover configurable identity headers, request-id
  fallback, JSON content-type checks, and path suffix matching.
- Request parsing: `ParseOpenAIChatRequest`, `CurrentUserIntent`,
  `ReplaceOpenAIChatMessages`, and `BuildRequestDigest` cover OpenAI chat
  request parsing, unknown field preservation, latest user text extraction,
  body replacement, and stable digest construction.
- Response parsing: `ParseOpenAIChatResponse`, `ResponseUsage`,
  `ResponseFinishReason`, and `ContainsToolUse` cover non-streaming assistant
  content, usage, finish reason, and canonical or legacy tool-call detection.
- Streaming capture: `NewStreamCapture` and `AppendSSE` cover split SSE frames,
  CRLF/CR/LF line endings, `[DONE]`, role-only chunks, assistant content, and
  canonical or legacy tool-call deltas.
- Event/logging utilities: `NewEventEnvelope`, `BuildRedisStreamXADD`,
  `FailOpenLogFields`, and `RedactForLog` cover generic event ids, Redis Stream
  command assembly, fail-open fields, credential-key redaction, and
  caller-provided sensitive value redaction.

## Required Shared Extensions

- Add a digest convenience helper from a parsed `OpenAIChatRequest`, such as
  `RequestDigestInputFromOpenAIChatRequest` or `BuildOpenAIChatRequestDigest`,
  so callers do not duplicate the full field mapping into `RequestDigestInput`.
- Extend `StreamCapture` to retain final streamed `usage` and expose it through
  a `Usage()` accessor. The memory event tests require streamed usage when the
  upstream includes usage on the final SSE chunk.
- Extend `StreamCapture` tool-call detection to treat `finish_reason` values
  `tool_calls` and `function_call` as tool-use signals, matching non-streaming
  response behavior.
- Add policy-free configurable JSON-path checks for additional tool-call
  locations so `tool_calls_from` can support canonical paths and route-specific
  vendor paths in both non-streaming responses and stream chunks.

## Not Shared Scope

- Memory modes, route policy, no-store policy, capture eligibility, and
  raw-content omission rules.
- Request header mutation, plugin gating decisions, and fail-open request
  control flow.
- Redis recent-memory key construction, lookup timing, schema validation,
  expiration checks, tenant/consumer/policy/session matching, role filtering,
  and value-size filtering.
- Console assemble request and response schema, internal auth handling, timeout
  fallback, response decisions, and logging policy for raw assemble payloads.
- Final memory injection order, high-priority message preservation rules, and
  duplicate recent-window avoidance.
- MemoryEvent schema beyond the generic envelope, including schema version,
  session, route/model objects, request path, plugin version, memory mode,
  policy version, status eligibility, usage normalization, and raw content.
- Durable memory ownership: embeddings, vector recall, PostgreSQL writes,
  retention, deletion, repair, management APIs, pricing, billing, and Console
  materialization ownership.
