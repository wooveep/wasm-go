## 1. Governance and Scope

- [x] 1.1 Run `openspec validate add-ai-cache-thin-plugin --strict` before implementation starts.
- [x] 1.2 Read `docs/ai-cache-thin-plugin-design.md`, `docs/2026-06-26-ai-cache-memory-shared-runtime-design.md`, this change's `proposal.md`, `design.md`, and specs before touching code.
- [ ] 1.3 Run GitNexus upstream impact analysis before editing each function, method, type, or shared helper involved in `ai-cache`, `ai-memory`, or `ai-billing`.
- [ ] 1.4 Treat production embedding generation, vector search, PostgreSQL writes, billing settlement, and provider credential ownership as out of scope for the gateway plugin.
- [ ] 1.5 Keep unrelated dirty files and unrelated existing OpenSpec changes out of this implementation diff.

## 2. Tests First

- [x] 2.1 Add config parsing tests for Redis materialized lookup, optional Console lookup, Redis Stream event settings, route policy, identity headers, cache scope, policy version, and default fail-open behavior.
- [x] 2.2 Add Redis replay tests for hit, miss, invalid JSON, unsupported schema version, expired records, scope mismatch, route mismatch, model mismatch, and policy version mismatch.
- [x] 2.3 Add Console lookup tests for disabled lookup, enabled lookup after Redis miss, hit validation, miss, timeout, and failure.
- [x] 2.4 Add cache-hit replay tests for OpenAI-compatible non-streaming response fields and supported streaming replay payloads.
- [x] 2.5 Add billing fact tests proving cache replay sets trusted `upstream_invoked=false` facts without adding billing fields to the user response body.
- [x] 2.6 Add upstream response capture tests for non-streaming and streaming assistant text, usage, finish reason, tool-call detection, no-store, sensitive, and failed-response gates.
- [x] 2.7 Add `CacheEvent` payload and redaction tests, including Redis Stream `XADD` failure fail-open behavior.
- [x] 2.8 Add shared helper tests covering identity extraction, session/request id extraction, OpenAI message parsing, current user intent extraction, request digest helpers, stream capture, event envelope helpers, and safe logging.
- [x] 2.9 Add memory-aware cache tests for consumer-scoped cache, memory policy/digest-aware keys or policy versions, and configured bypass on memory-enabled routes.
- [x] 2.10 Add legacy compatibility regression tests proving old text-only or embedding/vector paths are not used by the production thin-plugin default path.

## 3. Shared Protocol Helpers

- [x] 3.1 Create or update a shared Go package for policy-free AI protocol helpers used by `ai-cache` and `ai-memory`.
- [x] 3.2 Choose a repository-local shared helper package path, such as `pkg/ai/sessionctx` or `extensions/pkg/chatctx`, following existing repository conventions.
- [x] 3.3 Move tenant, consumer, configurable identity header names, session id, request id, path suffix, and JSON content-type extraction into shared helpers where safe.
- [x] 3.4 Move OpenAI-compatible message parsing, latest user intent extraction, request digest helpers, request body replacement helpers for memory injection, tool-call detection, usage extraction, and finish-reason extraction into shared helpers where safe.
- [x] 3.5 Move streaming and non-streaming assistant text capture into shared helpers while preserving existing `ai-memory` behavior.
- [x] 3.6 Add shared Redis Stream envelope, fail-open logging, and sensitive-value redaction helpers without adding cache policy, memory policy, vector code, pricing, or settlement logic.
- [x] 3.7 Verify shared helper adoption does not introduce shared Redis streams, Redis key prefixes, vector namespaces, payload schemas, retention policies, deletion policies, management APIs, or authorization checks.

## 4. Thin Cache Configuration and Lookup

- [x] 4.1 Implement thin-plugin config structs and validation for materialized Redis lookup, optional Console lookup, Redis Stream event emission, route policy, cache scope, policy version, enabled path suffixes, and fail policy.
- [x] 4.2 Implement global external-target inheritance and route-level behavior overrides.
- [x] 4.3 Implement request gating for enabled paths, JSON content type, missing identity, no-store, sensitive, and configured cache bypass.
- [x] 4.4 Implement scoped request digest and materialized Redis key construction with tenant, optional consumer, route, model, request digest, and policy version inputs.
- [x] 4.5 Implement Redis materialized record loading and strict replay validation.
- [x] 4.6 Implement optional Console `/internal/cache/lookup` callout with short timeout and the same replay validation as Redis records.
- [x] 4.7 Ensure Redis lookup, Console lookup, invalid records, misses, and timeouts all fail open to upstream.

## 5. Cache Replay and Response Capture

- [x] 5.1 Implement OpenAI-compatible non-streaming replay from structured materialized records.
- [x] 5.2 Implement supported streaming replay only for records explicitly marked stream-replayable.
- [x] 5.3 Set trusted internal cache-hit and `upstream_invoked=false` facts on replay without exposing proprietary billing fields in the response body.
- [x] 5.4 Preserve upstream request flow on miss or dependency failure.
- [x] 5.5 Capture eligible upstream response facts for non-streaming responses.
- [x] 5.6 Capture eligible upstream response facts for streaming responses and stream-done fallback paths.
- [x] 5.7 Prevent cacheable raw content emission when response parsing fails or policy marks the response as no-store or sensitive.

## 6. CacheEvent Emission

- [x] 6.1 Define `CacheEvent` payload structs with required event, identity, route, model, digest, policy, status, stream, gate, timing, and plugin version fields.
- [x] 6.2 Include permitted user content, assistant content, usage, finish reason, and safe provider/runtime facts only when policy allows them.
- [x] 6.3 Redact authorization headers, API keys, internal bearer tokens, Redis credentials, upstream provider credentials, and sensitive raw content.
- [x] 6.4 Emit one JSON `CacheEvent` to Redis Stream `cache:events` field `event` after eligible upstream responses finish.
- [x] 6.5 Keep user responses unchanged when Redis Stream delivery fails and log only safe diagnostic facts.

## 7. ai-memory and ai-billing Integration

- [x] 7.1 Ensure memory-enabled routes use consumer-scoped cache, memory policy/digest-aware cache keys or policy versions, or cache bypass until Console materializes memory-aware replay records.
- [x] 7.2 Document and test expected plugin ordering where `ai-memory` runs before `ai-cache` when memory affects the answer.
- [x] 7.3 Update `ai-billing` to consume trusted gateway-controlled cache replay facts.
- [x] 7.4 Update `ai-billing` event serialization and tests so upstream provider invocations emit `upstream_invoked=true` and trusted cache replays emit `upstream_invoked=false`.
- [x] 7.5 Add spoofing tests proving user-supplied request or response body fields cannot control `upstream_invoked`.

## 8. Documentation and Compatibility

- [x] 8.1 Update `extensions/ai-cache` documentation for thin-plugin responsibilities, config, Redis materialized records, optional Console lookup, fail-open behavior, replay validation, and `CacheEvent` emission.
- [x] 8.2 Update Chinese and English plugin docs that describe legacy `ai-cache` behavior so production structured replay is clearly distinguished from text-only compatibility.
- [x] 8.3 Document Console-owned responsibilities: materialization, embedding, vector search, durable replay records, semantic lookup, pricing, settlement, repair, deletion, and reconciliation.
- [x] 8.4 Document shared backend isolation rules for PostgreSQL tables, Redis streams, Redis key prefixes, vector collections, payload schemas, retention, deletion, management APIs, and authorization checks.
- [ ] 8.5 Document Console-managed internal AI Route usage for platform-owned cache and memory model work, including `internal_cost` attribution.
- [ ] 8.6 Document memory-aware cache policy options and recommended plugin ordering with `ai-memory`.
- [ ] 8.7 Document billing interaction through trusted `upstream_invoked` facts without user-visible billing response fields.
- [ ] 8.8 Mark existing online embedding/vector behavior as legacy or explicit non-production compatibility if it remains in code.

## 9. Verification

- [ ] 9.1 Run focused `ai-cache` Go tests.
- [ ] 9.2 Run focused shared helper and `ai-memory` regression tests.
- [ ] 9.3 Run focused `ai-billing` Go tests.
- [ ] 9.4 Run `openspec validate add-ai-cache-thin-plugin --strict`.
- [ ] 9.5 Run `git diff --check`.
- [ ] 9.6 Run `graphify update .` after code or documentation changes are complete.
- [ ] 9.7 Run GitNexus `detect_changes` before handoff or commit and review affected symbols and execution flows.
