## Context

`extensions/ai-memory` currently contains the design document but no runtime implementation. The intended architecture matches the thin-plugin direction already used for `ai-cache`: the gateway handles request/response protocol mechanics and short-latency hot-state reads, while Console owns durable data, model work, workers, repair, deletion, and policy.

The repository already has a shared AI protocol helper package under `pkg/ai/sessionctx` with identity extraction, path/content-type gating, OpenAI chat parsing, request digest helpers, message replacement, response capture, event envelope helpers, Redis Stream command construction, and safe redaction helpers. `ai-memory` should build on that package instead of copying `ai-cache` parsing and stream logic.

The plugin is latency-sensitive and must preserve availability. Redis recent-memory reads, Console assemble calls, request body rewriting, response capture, and Redis Stream event delivery are optimization paths; failures must not block the client request.

## Goals / Non-Goals

**Goals:**

- Add a thin `ai-memory` Wasm-Go plugin under `extensions/ai-memory`.
- Inject Console-approved memory context into OpenAI-compatible chat requests.
- Capture assistant responses for non-streaming and streaming flows.
- Emit one safe `MemoryEvent` to Redis Stream after response completion.
- Use Console-derived Redis recent memory as optional low-latency fallback.
- Optionally call Console `POST /internal/memory/assemble` with a short timeout for digest and semantic memory modes.
- Reuse policy-free shared helpers for protocol mechanics, response capture, event envelopes, fail-open logging, and redaction.
- Keep memory data isolated from `ai-cache` data and from customer billing events.

**Non-Goals:**

- Do not write PostgreSQL from the gateway plugin.
- Do not generate summaries, embeddings, vector indexes, or semantic recall inside the plugin.
- Do not call provider model APIs for memory work from the plugin.
- Do not own retention, deletion, repair, DLQ, management APIs, or billing settlement.
- Do not introduce a shared runtime plugin or shared business state for `ai-cache` and `ai-memory`.
- Do not log raw prompts, answers, credentials, authorization headers, Redis passwords, or internal bearer tokens.

## Decisions

### Decision: Implement `ai-memory` as a thin gateway plugin

The plugin will run only on configured AI path suffixes and JSON requests. It will read tenant and consumer identity, parse the current OpenAI-compatible request, optionally assemble memory, rewrite the request body, capture the response, and emit a `MemoryEvent`.

Rationale: this keeps memory behavior close enough to the request path for injection and response observation without making the gateway responsible for durable memory.

Alternative considered: implement memory as a Console-only feature. That avoids gateway code but cannot inject memory before upstream invocation unless every AI request is proxied through Console synchronously.

### Decision: Keep Console as the durable memory owner

Console owns PostgreSQL persistence, recent-window materialization, daily digest generation, semantic recall, embeddings, vector indexing, deletion, repair, management APIs, and backend model cost attribution. The plugin emits facts and consumes short-lived derived state only.

Rationale: memory policy, model work, and durable data lifecycle need database transactions, worker retries, schema migrations, authorization checks, and repair tooling that do not belong in a Wasm request filter.

Alternative considered: write memory records directly from the plugin. That would require durable-state semantics, retry, idempotency, schema evolution, and deletion policy in the gateway.

### Decision: Reuse and extend `pkg/ai/sessionctx`

`ai-memory` should use the shared package for identity extraction, path suffix and JSON gates, OpenAI message parsing, latest user intent extraction, request digest helpers, message replacement, response capture, tool-call detection, usage/finish-reason extraction, event envelope creation, Redis Stream `XADD` construction, and redaction.

The shared package must stay policy-free. It may expose protocol facts and mechanical body handling; it must not know memory mode, cache policy, vector providers, pricing, billing settlement, retention, deletion, or persistence rules.

Rationale: `ai-cache` and `ai-memory` need identical protocol mechanics. Shared helpers reduce drift in stream parsing, tool-call detection, digest construction, and redaction.

Alternative considered: duplicate helpers in `extensions/ai-memory`. That would make implementation faster initially but would create two subtly different protocol stacks.

### Decision: Split global external targets from route behavior

Global `defaultConfig` will define external targets and identity defaults such as Redis Stream, recent-memory Redis, Console internal service, tenant header, consumer header, session header, request id header, enabled suffixes, and fail policy. Route-level `matchRules[].config` will define memory behavior such as mode, recent window, token budget, assemble timeout, inject role, semantic top K, response capture, no-store header, and extraction paths.

Rule-level overrides for external targets such as `redis_stream`, `recent_cache`, and `console_internal` are out of scope for the first version.

Rationale: external destinations are deployment-level concerns, while memory mode and budgets are route policy. This matches existing Wasm-Go plugin inheritance patterns and avoids accidental per-route credential sprawl.

Alternative considered: allow every field at rule level. That is more flexible but complicates validation, credential handling, and operator review.

### Decision: Use Redis recent memory as optional derived state

The plugin may read Console-derived recent memory from Redis using configured key prefixes. It must miss safely on invalid JSON, tenant mismatch, consumer mismatch, policy mismatch, expiration, unsupported roles, and oversized values.

Rationale: recent memory can provide low-latency context when Console assemble is disabled or times out, but Redis is not authoritative memory storage.

Alternative considered: require Redis recent memory for every memory-enabled request. That would make Redis hot-state availability a correctness dependency.

### Decision: Treat Console assemble as optional and bounded

Routes in `digest` or `semantic` mode may call `POST /internal/memory/assemble` with a short timeout. Responses may inject a memory message, recent messages, both, or skip. Timeout, non-200 response, invalid response, or dispatch failure falls back to Redis recent memory or no memory.

Rationale: Console owns recall and policy, but user request latency should stay bounded and fail open.

Alternative considered: make Console assemble mandatory for memory correctness. That would make Console availability part of the user response path.

### Decision: Preserve client system/developer priority and avoid duplicate history

When injecting memory, the plugin will preserve original client system or developer messages first, then add one Console-returned memory message when present, then safe recent user/assistant turns, then the remaining current request messages. If the current request already contains multiple user turns, the plugin will avoid blindly duplicating recent-window history.

Rationale: memory should augment the request without overriding caller intent or producing duplicated chat history.

Alternative considered: prepend all memory before all client messages. That is simpler but can change priority semantics and create surprising prompt behavior.

### Decision: Emit one `MemoryEvent` after response completion

After response completion, the plugin emits one JSON event to Redis Stream `memory:events` field `event`. The event includes envelope identity, tenant, consumer, optional session, route, model, request id, request path, request digest, memory mode, status code, stream flag, tool-call flag, safe usage, finish reason, timing, and plugin version. Raw user and assistant content is omitted when capture is disabled, no-store is present, parsing fails, status is ineligible, or policy marks content unsafe.

Rationale: a single event gives Console idempotent worker input without making event delivery part of the client response contract.

Alternative considered: write separate request and response events. That improves observability but complicates idempotency and partial-event repair for the first version.

### Decision: Keep `ai-cache` and `ai-memory` isolated

`ai-memory` uses `memory:events` and memory key prefixes such as `memory:recent:`. It must not share `ai-cache` Redis streams, key prefixes, vector namespaces, payload schemas, retention policies, deletion policies, management APIs, or authorization checks.

Rationale: shared helpers are acceptable; shared business state would couple replayable cache data to user memory and make deletion and retention boundaries harder to enforce.

Alternative considered: share a session-context storage layer. That would reduce storage duplication but introduces cross-feature policy coupling and plugin ordering dependencies.

### Decision: Document plugin ordering with cache and proxy

`ai-memory` should run after identity and quota plugins and before `ai-cache` and `ai-proxy` when memory can affect the answer. `ai-cache` should use consumer-scoped, memory-policy-aware, memory-digest-aware, or bypass behavior on memory-enabled routes until Console materializes memory-aware replay records.

Rationale: cache replay that ignores memory injection can return responses inconsistent with the route's memory policy.

Alternative considered: let cache run before memory by default. That improves cache hit rate but silently bypasses memory inputs.

## Risks / Trade-offs

- Console assemble tail latency -> keep assemble short-timeout and fail open to recent memory or no memory.
- Redis recent-memory drift -> validate schema, tenant, consumer, policy, expiration, roles, and size before injection.
- Prompt leakage in logs -> centralize redaction helpers and test raw prompt, answer, credential, and token non-disclosure.
- Duplicate or excessive history injection -> detect multi-turn current requests and enforce route token/turn budgets.
- Streaming parser edge cases -> test split SSE chunks, mixed line endings, `[DONE]`, role-only deltas, and tool-call deltas.
- Event loss on Redis Stream failure -> keep user response unchanged, log safe diagnostics, and rely on future turns or Console repair workflows.
- Cache-memory inconsistency -> document ordering and require conservative cache scoping, memory digest policy, or bypass.
- Shared helper coupling -> keep helpers protocol-only and require tests proving no cache or memory business policy enters `pkg/ai/sessionctx`.

## Migration Plan

1. Add `ai-memory` configuration structs, validation, and route matching tests.
2. Add or extend shared helper coverage needed by memory injection and event emission.
3. Implement request gating, identity extraction, OpenAI request parsing, digest construction, recent-memory loading, Console assemble callout, message injection, and request body replacement.
4. Implement response capture for non-streaming and streaming flows, tool-call detection, no-store behavior, and `MemoryEvent` construction.
5. Implement Redis Stream `XADD` event dispatch with fail-open logging.
6. Add plugin README/resource catalog docs and examples.
7. Validate with focused `ai-memory`, shared helper, and OpenSpec tests.

Rollback is to disable the `ai-memory` plugin or set route `memory_mode: off`. Dependency failures already fail open and should not require a rollback for user traffic.

## Open Questions

- Which exact session header should be the default for this plugin: the design document's `x-mse-session` or the existing shared helper default `x-openclaw-session-key`?
- Should first-version `developer` injection be implemented immediately, or should it be rejected unless Console route policy explicitly declares developer-message compatibility?
- Which route/model fact source should be used when Higress runtime metadata is unavailable: request body `model`, route name properties, or configured fallback fields?
