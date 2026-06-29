# ai-cache-thin-plugin Specification

## Purpose
TBD - created by archiving change add-ai-cache-thin-plugin. Update Purpose after archive.
## Requirements
### Requirement: Thin cache plugin configuration
`ai-cache` SHALL support a thin-plugin configuration model for Redis materialized lookup, optional Console lookup, Redis Stream event emission, route policy, identity headers, cache scope, policy version, enabled path suffixes, and fail-open behavior.

#### Scenario: Global external targets are inherited by route rules
- **WHEN** `ai-cache` global config defines Redis materialized lookup, Console lookup, and Redis Stream event targets
- **AND** a matched route rule omits those external targets
- **THEN** the route SHALL inherit the global external target configuration

#### Scenario: Route policy controls online lookup behavior
- **WHEN** a matched route rule disables Redis lookup, Console lookup, or cache replay
- **THEN** `ai-cache` SHALL skip the disabled online cache behavior and continue the request upstream

#### Scenario: Fail policy defaults to open
- **WHEN** thin-plugin config omits `fail_policy`
- **THEN** `ai-cache` SHALL use `open`
- **AND** cache dependency failures SHALL NOT block the user response

#### Scenario: Supported requests are gated by path and content type
- **WHEN** a request path is not enabled by cache policy or the request is not supported JSON content
- **THEN** `ai-cache` SHALL skip cache lookup and continue upstream

### Requirement: Redis materialized replay lookup
`ai-cache` SHALL use Redis as the primary production replay source and SHALL read structured materialized replay records keyed by scope-relevant inputs.

#### Scenario: Materialized key includes scope inputs
- **WHEN** `ai-cache` builds a materialized replay key
- **THEN** the key SHALL include tenant, route, model, request digest, cache policy version, and consumer when the cache scope is consumer-scoped

#### Scenario: Valid materialized record can be replayed
- **WHEN** Redis returns a replay record with valid JSON, supported schema version, matching scope, matching route, matching model, matching policy version, usable OpenAI-compatible response, usage, finish reason, and unexpired hard expiration
- **THEN** `ai-cache` SHALL treat the record as a cache hit

#### Scenario: Invalid materialized record misses
- **WHEN** Redis returns invalid JSON, unsupported schema version, missing replay response, mismatched scope, mismatched route, mismatched model, mismatched policy version, expired hard expiration, or unusable usage facts
- **THEN** `ai-cache` SHALL treat the lookup as a cache miss and continue upstream

#### Scenario: Redis lookup failure fails open
- **WHEN** Redis materialized lookup times out or returns an error
- **THEN** `ai-cache` SHALL continue the request upstream
- **AND** the user response SHALL NOT expose Redis credentials or internal error details

### Requirement: Optional Console cache lookup
`ai-cache` SHALL support an optional route-policy-controlled Console lookup after Redis miss, and the lookup SHALL be a bounded optimization over precomputed or materialized records.

#### Scenario: Console lookup is called after Redis miss when enabled
- **WHEN** Redis materialized lookup misses
- **AND** the matched route policy enables Console lookup
- **THEN** `ai-cache` SHALL call Console `/internal/cache/lookup` with tenant, consumer, session, route, model, request digest, cache scope, and cache policy version facts

#### Scenario: Console lookup timeout fails open
- **WHEN** Console lookup exceeds its configured short timeout
- **THEN** `ai-cache` SHALL continue the request upstream

#### Scenario: Console lookup hit requires a valid replay record
- **WHEN** Console lookup returns a replay record
- **THEN** `ai-cache` SHALL apply the same structured replay validation used for Redis materialized records before replaying it

#### Scenario: Console lookup does not perform default online semantic work in the plugin
- **WHEN** the production thin-plugin path handles a request
- **THEN** `ai-cache` SHALL NOT generate embeddings or call vector databases from the gateway plugin

### Requirement: OpenAI-compatible cache-hit replay
`ai-cache` SHALL replay valid cache hits as OpenAI-compatible responses without adding proprietary billing fields to the user response body.

#### Scenario: Non-streaming replay preserves OpenAI response fields
- **WHEN** `ai-cache` replays a non-streaming cache hit
- **THEN** the response body SHALL preserve the replay record's user-visible model, choices, assistant content, finish reason, and usage

#### Scenario: Streaming replay preserves OpenAI stream compatibility
- **WHEN** `ai-cache` replays a structured streaming cache hit that is marked as stream-replayable
- **THEN** the response SHALL use OpenAI-compatible streaming payloads and completion markers

#### Scenario: Replay emits trusted billing facts
- **WHEN** `ai-cache` sends a cache-hit replay response without invoking upstream
- **THEN** it SHALL set trusted gateway facts indicating cache hit and `upstream_invoked=false`
- **AND** it SHALL NOT add proprietary billing fields to the user response body

#### Scenario: Cache miss resumes upstream
- **WHEN** Redis and optional Console lookup miss or fail
- **THEN** `ai-cache` SHALL resume the upstream request without mutating the user-visible request body except for configured shared protocol mechanics

### Requirement: Cache events are emitted after upstream responses
`ai-cache` SHALL emit a JSON `CacheEvent` to Redis Stream `cache:events` field `event` after eligible upstream responses finish.

#### Scenario: CacheEvent contains required safe facts
- **WHEN** an eligible upstream response finishes
- **THEN** `ai-cache` SHALL emit `event_id`, `idempotency_key`, `tenant`, `consumer`, `route`, `model`, `request_id`, `request_path`, `request_digest`, `cache_scope`, `cache_policy_version`, `status_code`, `is_stream`, `contains_tool_calls`, `no_store`, `sensitive`, `started_at_ms`, `ended_at_ms`, and `plugin_version`

#### Scenario: CacheEvent includes only permitted materialization inputs
- **WHEN** policy enables raw materialization inputs for cache materialization
- **THEN** `ai-cache` SHALL include only policy-permitted user content, assistant content, response usage, finish reason, and safe provider or runtime cluster facts in the `CacheEvent`

#### Scenario: Sensitive values are redacted from CacheEvent
- **WHEN** `ai-cache` builds a `CacheEvent`
- **THEN** the event SHALL NOT include authorization headers, API keys, internal bearer tokens, Redis credentials, upstream provider credentials, or raw sensitive content marked by policy

#### Scenario: Redis Stream failure fails open
- **WHEN** Redis Stream `XADD` times out or returns an error
- **THEN** `ai-cache` SHALL keep the user response unchanged and log only safe diagnostic facts

#### Scenario: Response parsing failure prevents cacheable content emission
- **WHEN** `ai-cache` cannot safely parse response content or usage
- **THEN** it SHALL NOT emit cacheable raw assistant content for materialization

### Requirement: Shared protocol helper package
`ai-cache` SHALL share policy-free protocol helpers with `ai-memory` through a Go package rather than a separate runtime plugin.

#### Scenario: Shared helpers cover protocol mechanics
- **WHEN** `ai-cache` and `ai-memory` need common protocol handling
- **THEN** shared helpers SHALL cover tenant and consumer extraction, configurable identity header names, session id extraction, request id extraction, OpenAI message parsing, current user intent extraction, request digest helpers, request body replacement helpers for memory injection, streaming and non-streaming assistant text capture, tool-call detection, safe usage extraction, event envelope helpers, safe logging, and fail-open helpers

#### Scenario: Shared helpers remain policy-free
- **WHEN** shared helper code is implemented
- **THEN** it SHALL NOT contain cache policy, memory policy, vector provider logic, pricing, billing settlement, retention policy, deletion policy, or durable materialization ownership

#### Scenario: Shared helpers do not create shared runtime state
- **WHEN** `ai-cache` and `ai-memory` use the shared helper package
- **THEN** they SHALL keep separate Redis stream names, Redis key prefixes, payload schemas, vector namespaces or collections, retention policies, deletion policies, management APIs, and authorization checks

### Requirement: Console backend work remains isolated from customer traffic
Console-owned cache and memory workers SHALL own heavy model work, durable state, derived Redis state, idempotency, retry, DLQ, and repair workflows outside the gateway plugin.

#### Scenario: Internal model work uses internal AI Routes
- **WHEN** Console performs platform-owned cache embedding, cache rerank, memory digest, memory embedding, or memory recall enhancement model calls
- **THEN** those calls SHALL use Console-managed internal AI Routes
- **AND** related cost attribution SHALL use internal cost events rather than ordinary customer usage statements

#### Scenario: Gateway plugins do not own backend persistence
- **WHEN** `ai-cache` or `ai-memory` handles a gateway request
- **THEN** the plugin SHALL NOT write authoritative PostgreSQL state, perform durable materialization, run worker retry or DLQ workflows, or use upstream provider credentials for platform-owned model work

### Requirement: Memory-aware cache behavior
`ai-cache` SHALL make cache replay behavior explicit when `ai-memory` also affects a route's upstream model request.

#### Scenario: Memory-aware route uses conservative cache policy
- **WHEN** `ai-cache` and `ai-memory` are both enabled on a route where memory affects the answer
- **THEN** `ai-cache` SHALL use consumer-scoped cache, include memory policy version and assembled memory digest in cache keys or policy version, or bypass cache until Console materializes memory-aware replay records

#### Scenario: Memory runs before cache lookup when required
- **WHEN** memory injection is expected to affect the upstream request
- **THEN** route configuration SHALL allow `ai-memory` to run before `ai-cache` lookup

#### Scenario: Cache before memory is explicit policy
- **WHEN** a route intentionally allows cache replay before memory injection
- **THEN** that behavior SHALL be treated as an explicit route policy choice rather than the default cache behavior

### Requirement: Legacy cache behavior is compatibility-only
Existing text-only replay, online embedding, and vector provider behavior SHALL NOT be part of the default production thin-plugin cache path.

#### Scenario: Legacy text-only cache is explicit compatibility
- **WHEN** an operator enables legacy text-only cache compatibility
- **THEN** `ai-cache` MAY use the existing text replay behavior
- **AND** documentation SHALL identify it as legacy or compatibility behavior outside the production structured replay path

#### Scenario: Online embedding and vector search are not default production behavior
- **WHEN** the production thin-plugin path handles a request
- **THEN** `ai-cache` SHALL NOT call embedding providers or vector databases unless an explicit non-production or legacy compatibility policy enables that path

