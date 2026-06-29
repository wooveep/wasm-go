# ai-memory-thin-plugin Specification

## Purpose
TBD - created by archiving change add-ai-memory-thin-plugin. Update Purpose after archive.
## Requirements
### Requirement: Memory plugin configuration
`ai-memory` SHALL support a thin-plugin configuration model with global external targets and route-level memory behavior.

#### Scenario: Global external targets are inherited by route rules
- **WHEN** `ai-memory` global config defines Redis Stream, recent-memory Redis, Console internal service, identity headers, enabled path suffixes, and fail policy
- **AND** a matched route rule omits those external target settings
- **THEN** the matched route SHALL inherit the global external target configuration

#### Scenario: Route config controls memory behavior
- **WHEN** a matched route rule configures `memory_mode`, `recent_window_turns`, `memory_token_budget`, `assemble_timeout_ms`, `inject_role`, `semantic_top_k`, `capture_response`, `no_store_header`, and extraction paths
- **THEN** `ai-memory` SHALL use those route-level values for the matched request without requiring route-level Redis or Console target configuration

#### Scenario: Fail policy defaults to open
- **WHEN** `fail_policy` is omitted
- **THEN** `ai-memory` SHALL use `open`
- **AND** memory dependency failures SHALL NOT block or alter the client response except for omitted memory injection or omitted event content

#### Scenario: Only open fail policy is accepted
- **WHEN** plugin configuration sets `fail_policy` to a value other than `open`
- **THEN** `ai-memory` SHALL reject the configuration

#### Scenario: Route-level external target overrides are rejected
- **WHEN** a route rule config contains `redis_stream`, `recent_cache`, or `console_internal`
- **THEN** `ai-memory` SHALL reject the rule configuration for the first version

#### Scenario: Memory modes are validated
- **WHEN** a route rule configures `memory_mode`
- **THEN** `ai-memory` SHALL accept only `off`, `recent-only`, `digest`, or `semantic`

### Requirement: Request gating and identity extraction
`ai-memory` SHALL run memory behavior only for supported AI JSON requests with trusted tenant and consumer identity.

#### Scenario: Supported requests are gated by path and content type
- **WHEN** a request path does not match configured AI path suffixes or the request content type is not JSON
- **THEN** `ai-memory` SHALL skip memory injection and continue the request unchanged

#### Scenario: Missing identity fails open
- **WHEN** an enabled AI request lacks the configured tenant header or configured consumer header
- **THEN** `ai-memory` SHALL skip memory injection and event content capture
- **AND** the request SHALL continue upstream

#### Scenario: Identity headers are configurable
- **WHEN** global config sets tenant, consumer, session, or request id header names
- **THEN** `ai-memory` SHALL extract request facts from those configured headers
- **AND** it SHALL default tenant and consumer header names to `x-mse-tenant` and `x-mse-consumer` when omitted

#### Scenario: Memory off bypasses behavior
- **WHEN** a matched route sets `memory_mode` to `off`
- **THEN** `ai-memory` SHALL skip recent-memory reads, Console assemble calls, request body replacement, and memory event raw content capture for that route

### Requirement: OpenAI request parsing and memory injection
`ai-memory` SHALL parse OpenAI-compatible chat requests, assemble safe memory context, and replace the request body only when memory injection succeeds.

#### Scenario: Current user intent and request digest are extracted
- **WHEN** `ai-memory` parses a supported OpenAI-compatible chat request body
- **THEN** it SHALL extract the latest user content and build a stable request digest from identity, route, model, and current request facts

#### Scenario: Client high-priority messages are preserved
- **WHEN** `ai-memory` injects memory into a request
- **THEN** the final messages array SHALL preserve original client `system` or `developer` messages before injected memory messages

#### Scenario: Memory message is inserted before current conversation messages
- **WHEN** Console assemble returns an injectable memory message
- **THEN** `ai-memory` SHALL insert that memory message after original high-priority client messages and before the remaining current request messages

#### Scenario: Recent messages are inserted when safe
- **WHEN** Redis recent memory or Console assemble returns safe recent `user` and `assistant` messages
- **THEN** `ai-memory` SHALL insert those recent messages after the memory message and before the remaining current request messages

#### Scenario: Multi-user-turn requests avoid duplicate recent history
- **WHEN** the current client request already contains multiple user turns
- **THEN** `ai-memory` SHALL NOT blindly inject duplicate recent-window history from Redis
- **AND** it MAY still use Console-returned digest or semantic memory when the route mode allows it

#### Scenario: Developer injection requires route compatibility
- **WHEN** a route config sets `inject_role` to `developer`
- **THEN** `ai-memory` SHALL use developer-role injection only when route policy marks the upstream route as developer-message compatible
- **AND** it SHALL otherwise reject the route config or fall back according to documented validation behavior

#### Scenario: Request body replacement failure fails open
- **WHEN** request JSON parsing, memory assembly, or body replacement fails
- **THEN** `ai-memory` SHALL continue upstream with the original request body
- **AND** it SHALL log only safe diagnostic facts

### Requirement: Redis recent memory fallback
`ai-memory` SHALL treat Redis recent memory as optional Console-derived runtime state.

#### Scenario: Recent memory is read in enabled modes
- **WHEN** a matched route uses `recent-only`, `digest`, or `semantic` mode and recent-memory Redis is configured
- **THEN** `ai-memory` SHALL attempt to read recent memory using the configured key prefix, tenant, consumer, and optional session facts

#### Scenario: Valid recent memory can be injected
- **WHEN** Redis recent memory contains valid JSON with supported schema version, matching tenant, matching consumer, matching policy version, unexpired timestamp, supported roles, and size within configured limits
- **THEN** `ai-memory` SHALL treat the messages as safe candidates for injection

#### Scenario: Invalid recent memory misses
- **WHEN** Redis recent memory has invalid JSON, unsupported schema version, tenant mismatch, consumer mismatch, expired data, policy mismatch, unsupported roles, or oversized values
- **THEN** `ai-memory` SHALL treat the recent-memory read as a miss
- **AND** it SHALL continue request handling without exposing raw Redis value content

#### Scenario: Redis recent read failure fails open
- **WHEN** the recent-memory Redis read times out or returns an error
- **THEN** `ai-memory` SHALL continue without recent memory

### Requirement: Console memory assemble callout
`ai-memory` SHALL support a bounded optional Console assemble call for digest and semantic memory modes.

#### Scenario: Assemble is called for digest and semantic modes
- **WHEN** a matched route uses `digest` or `semantic` mode and Console internal service is configured
- **THEN** `ai-memory` SHALL call `POST /internal/memory/assemble` with tenant, consumer, optional session, route, model, request id, request path, current question, messages digest, memory mode, recent window, token budget, semantic top K, and policy version facts

#### Scenario: Assemble is skipped for off and recent-only modes
- **WHEN** a matched route uses `off` or `recent-only` mode
- **THEN** `ai-memory` SHALL NOT call Console assemble

#### Scenario: Assemble decision controls injection
- **WHEN** Console assemble returns a valid response with decision `inject`, `recent_only`, `skip`, or `bypass`
- **THEN** `ai-memory` SHALL apply the returned decision to memory and recent-message injection

#### Scenario: Assemble timeout fails open
- **WHEN** Console assemble exceeds the configured timeout
- **THEN** `ai-memory` SHALL continue with valid Redis recent memory when available or continue without memory

#### Scenario: Assemble response content is never logged raw
- **WHEN** Console assemble returns memory messages, recent messages, trace, diagnostics, or an error response
- **THEN** `ai-memory` SHALL NOT log raw memory message content, raw recent message content, prompts, answers, credentials, or internal bearer tokens

### Requirement: Response capture
`ai-memory` SHALL capture assistant response facts for eligible non-streaming and streaming OpenAI-compatible responses.

#### Scenario: Non-streaming assistant content is captured
- **WHEN** an eligible non-streaming response contains assistant content at the configured response extraction path
- **THEN** `ai-memory` SHALL capture assistant content, finish reason, tool-call presence, status code, and safe usage when present

#### Scenario: Streaming assistant content is captured across chunks
- **WHEN** an eligible streaming response emits OpenAI-compatible SSE chunks
- **THEN** `ai-memory` SHALL accumulate assistant content across split chunks and line endings using `\r\n`, `\r`, or `\n`

#### Scenario: Streaming completion markers are handled
- **WHEN** a streaming response contains `[DONE]` alone or in the same buffer as final content
- **THEN** `ai-memory` SHALL finish capture without treating `[DONE]` as assistant content

#### Scenario: Role-only chunks do not corrupt content
- **WHEN** a streaming response chunk contains role metadata without content
- **THEN** `ai-memory` SHALL keep capture state valid and SHALL NOT append empty role metadata to assistant content

#### Scenario: Tool calls are detected
- **WHEN** non-streaming or streaming responses contain canonical OpenAI tool calls, legacy function calls, configured tool-call paths, or finish reasons indicating tool use
- **THEN** `ai-memory` SHALL set `contains_tool_calls` to `true`

#### Scenario: Capture can be disabled
- **WHEN** route config sets `capture_response` to `false` or the request contains the configured no-store header
- **THEN** `ai-memory` SHALL omit raw user and assistant content from the `MemoryEvent`
- **AND** it SHALL still emit safe operational facts when event emission is otherwise enabled

### Requirement: Memory events are emitted after response completion
`ai-memory` SHALL emit one JSON `MemoryEvent` to Redis Stream `memory:events` field `event` after eligible response completion.

#### Scenario: MemoryEvent contains required safe facts
- **WHEN** an eligible memory-enabled request completes
- **THEN** `ai-memory` SHALL emit `schema_version`, `event_id`, `idempotency_key`, `tenant`, `consumer`, `request_id`, `request_path`, `request_digest`, `memory_mode`, `status_code`, `is_stream`, `contains_tool_calls`, `started_at_ms`, `ended_at_ms`, and `plugin_version`

#### Scenario: MemoryEvent includes optional context facts
- **WHEN** session, route, model, policy version, usage, finish reason, or safe raw content are available and allowed by policy
- **THEN** `ai-memory` SHALL include those fields in the `MemoryEvent`

#### Scenario: MemoryEvent omits disallowed raw content
- **WHEN** response capture is disabled, no-store is present, parsing failed, content is unsafe by policy, or status code is ineligible
- **THEN** `ai-memory` SHALL omit `user_content` and `assistant_content` from the `MemoryEvent`

#### Scenario: Redis Stream event delivery fails open
- **WHEN** Redis Stream `XADD` dispatch fails, times out, or returns an error
- **THEN** `ai-memory` SHALL keep the user response unchanged
- **AND** it SHALL log only safe diagnostic facts

#### Scenario: Plugin does not trim stream or write DLQ
- **WHEN** `ai-memory` emits events to Redis Stream
- **THEN** it SHALL NOT trim the producer stream and SHALL NOT write gateway-owned DLQ entries in the first version

### Requirement: Memory ownership and isolation
`ai-memory` SHALL keep gateway memory behavior isolated from durable Console memory ownership and from `ai-cache` business state.

#### Scenario: Gateway plugin does not own durable memory
- **WHEN** `ai-memory` handles a request or response
- **THEN** it SHALL NOT write PostgreSQL, generate daily memory digests or durable memory summaries, generate embeddings, call vector databases, call provider model APIs for memory work, apply retention or deletion policy, calculate cost, debit balances, or create billing statements
- **AND** request digest construction for Console assemble and MemoryEvent idempotency SHALL remain a safe protocol fact rather than durable memory generation

#### Scenario: Memory data is isolated from cache data
- **WHEN** `ai-memory` stores or reads gateway hot data
- **THEN** it SHALL use memory-specific Redis streams, Redis key prefixes, payload schemas, vector namespaces or collections, retention policies, deletion policies, management APIs, and authorization boundaries

#### Scenario: Shared helpers remain policy-free
- **WHEN** `ai-memory` and `ai-cache` use shared AI protocol helper code
- **THEN** the shared helpers SHALL NOT contain cache policy, memory policy, vector provider logic, pricing, billing settlement, retention policy, deletion policy, or durable storage ownership

### Requirement: Plugin ordering and cache interaction
`ai-memory` SHALL define safe runtime ordering when used with identity, quota, cache, billing, and proxy plugins.

#### Scenario: Memory runs after identity and quota
- **WHEN** a route enables `ai-memory`
- **THEN** deployment documentation SHALL instruct operators to run `ai-memory` after identity and quota plugins so trusted tenant and consumer identity are available

#### Scenario: Memory runs before cache and proxy when memory affects the answer
- **WHEN** memory injection can affect the upstream model request
- **THEN** deployment documentation SHALL instruct operators to run `ai-memory` before `ai-cache` and `ai-proxy`

#### Scenario: Cache policy is conservative on memory-enabled routes
- **WHEN** `ai-memory` and `ai-cache` are both enabled on a route
- **THEN** the route SHALL use consumer-scoped cache, memory policy or digest-aware cache keys, or cache bypass until Console materializes memory-aware replay records

