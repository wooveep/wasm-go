## 1. Governance and Scope

- [x] 1.1 Run `openspec validate add-ai-memory-thin-plugin --strict` before implementation starts.
- [x] 1.2 Read `extensions/ai-memory/DESIGN.md`, `docs/2026-06-26-ai-cache-memory-shared-runtime-design.md`, this change's `proposal.md`, `design.md`, and specs before touching code.
- [x] 1.3 Run GitNexus upstream impact analysis before editing each function, method, type, or shared helper involved in `ai-memory`, `pkg/ai/sessionctx`, `ai-cache`, or resource docs.
- [ ] 1.4 Keep durable memory ownership, embedding, vector recall, PostgreSQL writes, retention, deletion, repair, management APIs, and billing settlement out of the gateway plugin.
- [ ] 1.5 Keep unrelated dirty files and unrelated existing OpenSpec changes out of this implementation diff.

## 2. Tests First

- [x] 2.1 Add config parsing tests for global Redis Stream, recent-memory Redis, Console internal service, identity headers, enabled suffixes, `fail_policy`, route memory behavior, and external-target inheritance.
- [x] 2.2 Add validation tests for unsupported `fail_policy`, invalid `memory_mode`, route-level `redis_stream`, route-level `recent_cache`, and route-level `console_internal`.
- [x] 2.3 Add request gating tests for path suffixes, JSON content type, missing tenant, missing consumer, and `memory_mode: off`.
- [x] 2.4 Add OpenAI request parsing and injection tests for latest user extraction, request digest stability, client system/developer message preservation, memory message insertion, safe recent message insertion, and multi-user-turn duplicate avoidance.
- [x] 2.5 Add Redis recent-memory tests for hit, miss, invalid JSON, unsupported schema version, tenant mismatch, consumer mismatch, policy mismatch, expiration, unsupported roles, oversized values, timeout, and failure.
- [x] 2.6 Add Console assemble tests for disabled modes, enabled `digest` and `semantic` modes, valid `inject`, `recent_only`, `skip`, and `bypass` decisions, timeout fallback, invalid response fallback, and safe logging.
- [x] 2.7 Add request body replacement fail-open tests proving parse, assemble, and replacement failures continue upstream with the original request body.
- [x] 2.8 Add non-streaming response capture tests for assistant content, usage, finish reason, status code, tool-call detection, no-store, disabled capture, and parse failure.
- [x] 2.9 Add streaming response capture tests for split SSE chunks, `\r\n`, `\r`, and `\n` line endings, `[DONE]`, role-only chunks, final-content buffers, tool-call deltas, usage, and finish reason.
- [x] 2.10 Add `MemoryEvent` payload tests covering required safe facts, optional session/route/model/policy/usage/finish fields, raw content omission gates, idempotency, stream name, field name, and Redis Stream dispatch failure fail-open behavior.
- [x] 2.11 Add privacy and isolation tests proving raw prompts, answers, credentials, authorization headers, Redis passwords, internal bearer tokens, cache streams, cache key prefixes, and cache payload schemas are not used or logged by `ai-memory`.
- [x] 2.12 Add resource catalog tests or checks for `resources/plugins/ai-memory` README, README_EN, `spec.yaml`, schema fields, inherited config examples, and secret placeholders.

## 3. Shared Protocol Helpers

- [x] 3.1 Review `pkg/ai/sessionctx` and identify only the helper extensions required by `ai-memory`.
- [x] 3.2 Extend shared helper tests before changing helper code for any missing message injection, stream capture, usage extraction, event envelope, Redis Stream, or redaction behavior.
- [x] 3.3 Implement missing policy-free helper behavior without adding cache policy, memory policy, vector provider logic, pricing, settlement, retention, deletion, or durable storage ownership.
- [x] 3.4 Verify shared helper changes preserve existing `ai-cache` behavior and tests.

## 4. ai-memory Configuration and Lifecycle

- [x] 4.1 Create the `extensions/ai-memory` Go module structure following local extension conventions.
- [x] 4.2 Implement config structs and validation for `redis_stream`, `recent_cache`, `console_internal`, identity headers, path suffixes, fail policy, and route memory behavior.
- [x] 4.3 Implement global default config plus route-level behavior override resolution.
- [x] 4.4 Implement request header handling for path/content-type gating, identity extraction, `memory_mode: off`, body buffering, `Accept-Encoding` removal, and `Content-Length` removal when body replacement may occur.
- [x] 4.5 Implement request context storage for tenant, consumer, session, request id, route, model, request path, memory mode, policy version, start time, stream flag, no-store, and request digest facts.

## 5. Memory Assembly and Request Injection

- [x] 5.1 Implement OpenAI-compatible request body parsing, latest user content extraction, and stable digest construction.
- [x] 5.2 Implement Redis recent-memory key construction, loading, validation, and miss behavior.
- [x] 5.3 Implement Console assemble request and response structs for `POST /internal/memory/assemble`.
- [x] 5.4 Implement bounded Console assemble callout for `digest` and `semantic` modes with fallback to Redis recent memory or no memory.
- [x] 5.5 Implement final message assembly order: original high-priority messages, Console memory message, safe recent messages, then remaining current request messages.
- [x] 5.6 Implement duplicate recent-window avoidance for current requests that already contain multiple user turns.
- [x] 5.7 Implement `inject_role` validation and developer-role compatibility behavior.
- [x] 5.8 Implement request body replacement and upstream resume, with fail-open behavior on every parse, assemble, validation, or replacement error.

## 6. Response Capture and MemoryEvent Emission

- [x] 6.1 Implement non-streaming response capture for assistant content, usage, finish reason, status code, and tool-call detection.
- [x] 6.2 Implement streaming SSE capture for split chunks, line ending variants, `[DONE]`, role-only chunks, tool-call deltas, usage, and finish reason.
- [x] 6.3 Implement no-store and disabled-capture gates that omit raw user and assistant content while preserving safe operational facts.
- [x] 6.4 Define `MemoryEvent` structs with schema version, envelope, identity, request, route, model, policy, status, stream, tool-call, usage, finish, timing, and plugin version fields.
- [x] 6.5 Implement `MemoryEvent` raw content omission rules for disabled capture, no-store, parse failure, unsafe content, and ineligible status.
- [x] 6.6 Emit one JSON `MemoryEvent` through Redis Stream `XADD` to `memory:events` field `event` after response completion.
- [x] 6.7 Ensure Redis Stream dispatch failures keep the user response unchanged and log only safe diagnostics.

## 7. Documentation and Resource Catalog

- [x] 7.1 Update `extensions/ai-memory` README documentation for responsibilities, configuration, request flow, Console assemble, Redis recent memory, response capture, events, fail-open behavior, privacy, and plugin ordering.
- [x] 7.2 Add `resources/plugins/ai-memory/README.md`, `README_EN.md`, and `spec.yaml`.
- [x] 7.3 Document Console-owned responsibilities and gateway non-goals in extension and resource documentation.
- [x] 7.4 Document isolation from `ai-cache` streams, key prefixes, vector namespaces, payload schemas, retention, deletion, management APIs, and authorization checks.
- [x] 7.5 Document recommended ordering after identity/quota and before `ai-cache` and `ai-proxy`.
- [x] 7.6 Add resource schema examples showing global external targets in `defaultConfig` and route memory behavior in `matchRules[].config`.

## 8. Verification

- [x] 8.1 Run focused shared helper tests.
- [x] 8.2 Run focused `ai-memory` Go tests.
- [x] 8.3 Run focused `ai-cache` regression tests affected by shared helper changes.
- [x] 8.4 Run resource documentation/schema validation or focused checks.
- [x] 8.5 Run `openspec validate add-ai-memory-thin-plugin --strict`.
- [x] 8.6 Run `git diff --check`.
- [ ] 8.7 Run `graphify update .` after code or documentation changes are complete when the implementation phase modifies project files.
- [ ] 8.8 Run GitNexus `detect_changes` before handoff or commit and review affected symbols and execution flows.
