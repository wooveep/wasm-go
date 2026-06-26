## Why

The current `ai-cache` plugin owns too much online work, including embedding and vector search paths that make gateway latency and production ownership harder to control. The plugin needs to become a thin, fail-open cache executor while Console owns materialization, semantic lookup, replay persistence, and final billing settlement.

## What Changes

- Redesign `ai-cache` as a lightweight online cache executor that reads structured materialized replay records from Redis and optionally asks Console for a short-timeout lookup over precomputed records.
- Preserve OpenAI-compatible cache-hit responses for both non-streaming and supported streaming replay while failing open to upstream on cache miss, invalid replay records, dependency errors, or lookup timeout.
- Emit safe `CacheEvent` records to Redis Stream after eligible upstream responses so Console workers can asynchronously materialize future cache hits.
- Stop treating production embedding generation, vector database calls, PostgreSQL writes, persistent replay records, customer pricing, upstream cost calculation, balance settlement, and provider credentials as gateway plugin responsibilities.
- Add shared Go protocol helpers for `ai-cache` and `ai-memory` without introducing another runtime plugin.
- Define memory-aware cache behavior so routes using both `ai-memory` and `ai-cache` either scope cache keys to memory inputs, include memory policy/version digests, or bypass cache until Console materializes memory-aware replay records.
- Extend billing-event behavior so trusted gateway facts from cache replay can produce `upstream_invoked=false` without adding proprietary billing fields to user responses.
- Update tests and documentation for the thin-plugin configuration, replay validation, fail-open behavior, event redaction, and legacy compatibility boundaries.

## Capabilities

### New Capabilities
- `ai-cache-thin-plugin`: Thin online `ai-cache` replay, lookup, response capture, event emission, shared protocol helper, and memory-aware cache behavior.

### Modified Capabilities
- `ai-billing-events`: Billing events consume trusted cache replay facts and emit whether upstream was invoked for cache-hit requests.

## Impact

- Affected code: `extensions/ai-cache`, shared helper packages used by `ai-cache` and `ai-memory`, focused `ai-billing` event fact handling, plugin tests, and AI cache documentation.
- Affected runtime behavior: cache hits may short-circuit upstream only when a structured materialized replay record is valid for tenant, consumer scope, route, model, request digest, and policy version; all cache dependency failures fail open.
- Affected APIs: OpenAI-compatible chat/replay responses, plugin configuration for Redis materialized lookup, optional Console lookup, Redis Stream event emission, and cache-related gateway headers/log facts.
- Dependencies: Redis remains the hot replay and event transport; Console becomes the owner of precomputed lookup, materialization, semantic cache work, replay persistence, and billing settlement inputs.
- Compatibility: existing text-only cache behavior may remain as a legacy local compatibility path, but production Modelfusion cache replay uses structured materialized records.
