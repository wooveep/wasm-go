## Context

The existing `ai-cache` plugin contains online cache execution, embedding providers, vector providers, and text-template replay behavior in the gateway path. That makes production latency sensitive to semantic cache work and mixes gateway responsibilities with Console responsibilities.

The target architecture is the same thin-plugin pattern used by the `ai-memory` design and the supplemental shared-runtime design in `docs/2026-06-26-ai-cache-memory-shared-runtime-design.md`: the gateway plugin handles request/response protocol mechanics and emits events, while Console owns durable state, asynchronous materialization, embedding, semantic recall, indexing, repair, deletion, and settlement workflows.

The implementation must preserve user request availability. Redis materialized lookup, optional Console lookup, and Redis Stream event delivery are optimization paths and must fail open unless a future explicit policy changes that contract.

## Goals / Non-Goals

**Goals:**

- Keep gateway cache lookup latency predictable.
- Replay only structured, validated, materialized cache records in the production path.
- Preserve OpenAI-compatible replay responses and token usage facts.
- Emit safe `CacheEvent` records for Console workers to materialize future hits.
- Provide trusted cache replay facts that let `ai-billing` report `upstream_invoked=false`.
- Share protocol parsing, response capture, Redis Stream envelope, and safe logging helpers with `ai-memory`.
- Define clear behavior when `ai-cache` and `ai-memory` are both enabled.

**Non-Goals:**

- Do not write PostgreSQL from the plugin.
- Do not generate embeddings or call vector databases in the production cache path.
- Do not calculate customer prices, discounts, upstream costs, balances, or billing settlement.
- Do not store upstream provider credentials in `ai-cache`.
- Do not make cache event delivery failures affect the user response.
- Do not introduce a shared runtime plugin for `ai-cache` and `ai-memory`.

## Decisions

### Decision: Make Redis materialized records the primary replay source

`ai-cache` will first read a structured replay record from Redis using a key derived from Console-projected policy inputs: tenant, optional consumer scope, route, model, request digest, and policy version. The record is valid only when schema version, scope, route, model, policy version, response body, usage, finish reason, `soft_expires_at`, and `hard_expires_at` pass validation.

Rationale: Redis keeps the gateway online path fast and avoids doing semantic cache work in the request. Structured records preserve replay and billing facts better than legacy text-only values.

Alternative considered: continue generating embeddings and querying vector databases online. This preserves more cache autonomy inside the plugin, but it increases tail latency and keeps production materialization ownership in the gateway.

### Decision: Treat Console lookup as optional and short-timeout

After Redis miss, the plugin may call Console `/internal/cache/lookup` only when route policy enables it. The default lookup contract reads precomputed or materialized results, uses a short timeout such as 50-100 ms, and fails open on miss, timeout, or error.

Rationale: Console can own cache policy and precomputed lookup while the gateway keeps latency bounded. Cache correctness must not depend on a synchronous Console call.

Alternative considered: make Console lookup mandatory for correctness. That would make Console availability a user-response dependency and would violate the fail-open gateway contract.

### Decision: Emit `CacheEvent` after eligible upstream responses

When the request reaches upstream and the response is eligible, `ai-cache` captures request facts, assistant text, usage, finish reason, and safe runtime facts, then writes one JSON `CacheEvent` to Redis Stream `cache:events` field `event`. Sensitive data, credentials, authorization headers, Redis credentials, upstream provider credentials, and internal bearer tokens are never included.

Rationale: event emission lets Console materialize replay records asynchronously without blocking user traffic or giving the gateway durable storage responsibility.

Alternative considered: write materialized records directly from the plugin. That would require stronger state ownership, retry, idempotency, persistence, and schema migration behavior in the gateway.

### Decision: Use trusted gateway facts for cache-hit billing

On replay, `ai-cache` will set trusted request context or headers/log facts consumed by `ai-billing`, including that upstream was not invoked. `ai-billing` will emit `upstream_invoked=false` in its event without adding proprietary billing fields to the user response body.

Rationale: billing settlement needs to distinguish cache replay from provider invocation, but users should still receive OpenAI-compatible responses.

Alternative considered: add billing metadata to the replay response body. That leaks internal accounting details into user-visible API payloads and breaks compatibility.

### Decision: Extract a policy-free shared protocol package

Shared helpers should cover identity extraction, configurable tenant and consumer headers with `x-mse-tenant` and `x-mse-consumer` defaults, session/request id extraction, OpenAI message parsing, latest user intent extraction, request digest helpers, request body replacement helpers for memory injection, streaming and non-streaming assistant text capture, tool-call detection, safe usage extraction, event envelope helpers, safe logging, fail-open helpers, and short-timeout Console callout mechanics.

The package must not contain cache policy, memory policy, vector code, billing settlement, pricing, deletion, retention, or durable state ownership.

The exact package path should follow local repository conventions during implementation. Candidate shapes from the supplemental design are `pkg/ai/sessionctx` or `extensions/pkg/chatctx`.

Rationale: `ai-cache` and `ai-memory` need the same protocol mechanics, and duplicating stream parsing or redaction logic creates inconsistency.

Alternative considered: copy helpers into each plugin. That avoids package design work but makes stream parsing, tool-call detection, and redaction behavior drift.

### Decision: Share helper code, not business storage or a runtime plugin

`ai-cache` and `ai-memory` will keep separate PostgreSQL tables, Redis stream names, Redis key prefixes, vector collections or namespaces, payload schemas, retention policies, deletion policies, management APIs, and authorization checks. The shared package exposes protocol facts and utilities only; each plugin decides how to use them.

Rationale: shared code reduces duplicated parsing and fail-open logic without adding plugin-order coupling or cross-contaminating cache replay data with memory data.

Alternative considered: introduce a standalone session context plugin or shared runtime component. That centralizes state but adds ordering coupling and creates a new business-state boundary in the gateway.

### Decision: Align backend model work through Console-managed internal AI Routes

Heavy model work owned by Console, such as cache embedding, future cache rerank, memory digest, memory embedding, or later recall enhancement, should use Console-managed internal AI Routes. These internal calls should emit `event_kind=internal_cost` for platform attribution and must not become ordinary customer usage statements.

Rationale: platform-owned AI work needs cost attribution without making internal service accounts look like customer billing principals.

Alternative considered: let gateway plugins or workers call provider models directly with plugin-owned credentials. That increases credential exposure and weakens the separation between customer traffic and platform maintenance work.

### Decision: Make memory-aware cache behavior explicit

When `ai-memory` and `ai-cache` both run on a route, `ai-memory` should run before cache lookup when memory affects the upstream request. First-version policies are limited to consumer-scoped cache, keys or policy versions that include memory policy version and assembled memory digest, or cache bypass for memory-enabled routes until Console materializes memory-aware replay records.

Rationale: replaying a response that ignores memory injection can return an answer inconsistent with the route's memory policy.

Alternative considered: allow cache to run before memory by default. That improves cache hit rate, but silently bypasses memory input and makes behavior hard to reason about.

### Decision: Keep old text/vector paths only as explicit compatibility

Existing text-only replay and embedding/vector code may remain as local, legacy, or experimental compatibility paths during migration, but the production thin-plugin path does not use online embeddings or vector search by default.

Rationale: this lowers migration risk while making production ownership and latency boundaries clear.

Alternative considered: remove legacy code immediately. That simplifies implementation but raises compatibility risk for existing local deployments.

## Risks / Trade-offs

- Redis replay schema drift -> validate schema version and policy version before replay and fail open on mismatch.
- Console lookup tail latency -> keep lookup disabled by default or route-policy controlled with a short timeout.
- Event loss during Redis Stream failure -> fail open for users, log safely, and rely on future upstream responses for rematerialization.
- Memory-aware cache miss rate -> start with conservative scoping or bypass, then let Console materialize memory-aware records.
- Billing fact spoofing -> use trusted internal request context or gateway-controlled headers only, not user-supplied response body fields.
- Internal model cost confusion -> route Console-owned model work through internal AI Routes and emit `internal_cost` events rather than customer usage statements.
- Legacy path confusion -> document production thin-plugin defaults and mark embedding/vector paths as legacy or explicit non-production compatibility.

## Migration Plan

1. Add thin-plugin configuration and structured replay validation behind route policy while keeping existing compatibility behavior available.
2. Add shared protocol helpers and move common parsing/capture/redaction logic used by `ai-cache` and `ai-memory`, choosing a repository-local package path that matches existing conventions.
3. Add Redis materialized lookup, optional Console lookup, OpenAI-compatible replay, and trusted billing facts.
4. Add upstream response capture and `CacheEvent` Redis Stream emission.
5. Update `ai-billing` to consume trusted cache replay facts and emit `upstream_invoked`.
6. Update docs and tests, then keep legacy embedding/vector behavior disabled by production defaults.

Rollback is to disable thin-plugin lookup policy or route cache bypass so requests continue upstream. Event emission failures already fail open and do not require rollback for user traffic.

## Open Questions

- Which exact trusted carrier should `ai-cache` use for billing facts: plugin context, gateway metadata, or an internal header stripped from user traffic?
- Should first-version streaming replay be enabled for all structured replay records, or only for records explicitly materialized as replayable streaming payloads?
- Which legacy config fields should be deprecated immediately versus retained as experimental compatibility knobs?
