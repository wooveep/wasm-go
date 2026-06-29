## Why

`ai-memory` needs a gateway implementation that can add relevant memory to AI requests and emit safe turn events without moving durable memory ownership into the Wasm plugin. The current repository has the memory design and shared `ai-cache` runtime direction documented, but it needs an OpenSpec contract before implementation starts.

## What Changes

- Add an `ai-memory` thin gateway plugin that gates AI requests, extracts tenant and consumer identity, reads optional recent memory, optionally asks Console to assemble digest or semantic memory, injects memory into OpenAI-compatible chat requests, captures assistant responses, and emits one `MemoryEvent` to Redis Stream.
- Keep durable memory responsibilities in Console: PostgreSQL persistence, recent-window materialization, daily digests, semantic recall, embeddings, vector indexing, deletion, repair, management APIs, and backend model cost attribution.
- Reuse or extend the policy-free shared AI protocol helper package used by `ai-cache` for identity extraction, OpenAI message parsing, request digest helpers, request body replacement, streaming and non-streaming response capture, tool-call detection, usage extraction, Redis Stream envelope helpers, fail-open behavior, and safe logging.
- Preserve strict isolation from `ai-cache`: separate Redis streams, Redis key prefixes, vector collections or namespaces, payload schemas, retention policies, deletion policies, management APIs, and authorization checks.
- Define fail-open behavior for missing identity, Redis recent read failure, Console assemble timeout or failure, JSON parsing failure, request body replacement failure, response capture failure, and Redis Stream event delivery failure.
- Add plugin documentation and resource catalog metadata for `ai-memory`, including configuration, ordering, privacy, and Console ownership boundaries.

## Capabilities

### New Capabilities
- `ai-memory-thin-plugin`: Gateway-side memory injection, response capture, event emission, Console assemble integration, Redis recent-memory fallback, shared helper use, isolation, privacy, and fail-open behavior.

### Modified Capabilities
- `ai-plugin-resource-docs`: Add operator-facing `ai-memory` resource documentation and schema metadata aligned with the thin-plugin runtime contract.

## Impact

- Affected code: new `extensions/ai-memory` plugin code, shared `pkg/ai/sessionctx` or equivalent AI protocol helpers, plugin tests, and resource catalog documentation.
- Affected runtime behavior: memory-enabled routes may rewrite OpenAI-compatible chat request bodies before upstream invocation and emit memory events after response completion; all memory dependency failures fail open.
- Affected APIs: plugin configuration, optional Console `POST /internal/memory/assemble` callout, Redis recent-memory reads, Redis Stream `memory:events` event payload, and OpenAI-compatible request/response parsing.
- Dependencies: Redis is used for recent-memory fallback and event transport; Console owns durable memory, assemble policy, semantic recall, digest generation, model work, and worker retry/DLQ behavior.
- Compatibility: `ai-memory` is a new plugin and should be ordered after identity/quota plugins and before plugins that may short-circuit or forward the AI request.
