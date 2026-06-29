---
title: AI Memory
keywords: [AI Gateway, AI Memory, ai-memory]
description: ai-memory thin gateway plugin configuration and runtime contract.
---

## Overview

`ai-memory` is a thin Higress Wasm-Go gateway plugin for OpenAI-compatible chat
traffic. It gates supported JSON AI requests, extracts trusted tenant and
consumer identity, optionally injects Console-derived memory into the request
body, captures assistant response facts, and emits one JSON `MemoryEvent` to
Redis Stream `memory:events` field `event` after the response completes.

The gateway plugin does not own durable memory. Console owns PostgreSQL
persistence, recent-window materialization, daily digest generation, semantic
recall, embeddings, vector indexing, deletion, repair, management APIs, and
backend model cost attribution.

## Responsibilities

The plugin handles only request-time and response-time gateway work:

- Request gating by path suffix and JSON `content-type`.
- Tenant, consumer, optional session, request id, route, model, request path,
  memory mode, policy version, stream flag, no-store flag, and request digest
  facts.
- Redis recent-memory fallback from Console-materialized recent-window state.
- Optional Console assemble callout to `POST /internal/memory/assemble` for
  `digest` and `semantic` modes.
- Safe request body replacement that preserves original high-priority client
  messages, injects one memory message when Console returns one, inserts safe
  recent `user` and `assistant` messages, and keeps the current request turns.
- Non-streaming and streaming assistant response capture, including usage,
  finish reason, response status, and tool-call detection.
- One `MemoryEvent` Redis Stream `XADD` after response completion.
- Fail-open behavior for Redis, Console, parsing, replacement, capture, and
  event dispatch failures.

The plugin does not write PostgreSQL, generate daily memory digests, generate
embeddings, call vector databases, call provider models for memory work,
enforce retention or deletion policy, expose memory management APIs, calculate
backend model cost, or create billing statements. It may build a request digest
as a safe protocol fact for Console assemble and MemoryEvent idempotency.

## Ownership Boundary

Console owns every durable or policy-heavy memory concern: PostgreSQL records,
recent-window materialization, daily digests, semantic recall, embeddings,
vector indexes, retention, deletion, repair, management APIs, authorization
checks, and backend model cost attribution.

The gateway plugin is intentionally not a memory system of record. It does not
persist conversations, generate memory artifacts, choose retention policy,
operate vector storage, repair missing turns, expose operator APIs, or settle
billing. It only consumes Console-derived runtime state and emits one
best-effort event for Console workers.

## Configuration

Global `defaultConfig` holds external targets and identity settings. Route
`matchRules[].config` holds memory behavior and inherits global external
targets. The first version rejects route-level `redis_stream`, `recent_cache`,
and `console_internal` overrides.

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.dns
    service_port: 6379
    database: 0
    timeout: 500
    stream: memory:events
  recent_cache:
    service_name: redis-stack-server.dns
    service_port: 6379
    database: 0
    timeout: 50
    key_prefix: memory:recent
  console_internal:
    service_name: modelfusion-console-api.dns
    service_port: 80
    assemble_path: /internal/memory/assemble
    timeout_ms: 100
    auth_token: <internal-bearer-token>
  tenant_header: x-mse-tenant
  consumer_header: x-mse-consumer
  session_header: x-mse-session
  request_id_header: x-request-id
  enable_path_suffixes:
    - /v1/chat/completions
    - /v1/messages
  fail_policy: open
matchRules:
  - ingress:
      - qwen
    config:
      memory_mode: semantic
      recent_window_turns: 6
      memory_token_budget: 1500
      assemble_timeout_ms: 100
      inject_role: system
      developer_compatible: false
      semantic_top_k: 3
      capture_response: true
      no_store_header: x-higress-ai-memory-no-store
      question_from: messages.@reverse.#(role=="user").content
      response_value_from: choices.0.message.content
      stream_value_from: choices.0.delta.content
      tool_calls_from:
        - choices.0.message.tool_calls
        - choices.0.message.function_call
        - choices.0.delta.tool_calls
        - choices.0.delta.function_call
      policy_version: memory-policy-v1
```

`memory_mode` accepts:

| Value | Behavior |
| --- | --- |
| `off` | Skips recent memory, Console assemble, request replacement, and raw event content capture. |
| `recent-only` | Uses only Redis recent-memory fallback. |
| `digest` | Allows Console assemble for digest memory. |
| `semantic` | Allows Console assemble for digest and semantic recall. |

`fail_policy` currently supports only `open`.

## Request Flow

1. Header phase checks configured path suffixes, JSON content type, tenant
   identity, and consumer identity. Unsupported or unauthenticated requests
   continue unchanged.
2. Body phase parses OpenAI-compatible messages, extracts the latest user
   content, stores request facts, and builds a stable request digest.
3. Recent-memory phase reads Redis key
   `memory:recent:<tenant>:<consumer>:<session>` using the configured
   `recent_cache.key_prefix`. Invalid, expired, mismatched, oversized, or
   unsupported recent records are treated as misses.
4. Console assemble phase runs only for `digest` and `semantic` modes. The
   plugin sends safe request facts to `POST /internal/memory/assemble` and
   applies valid `inject`, `recent_only`, `skip`, or `bypass` decisions.
5. Replacement phase rewrites the request body only after memory assembly is
   valid. Parse, assemble, validation, and replacement failures continue
   upstream with the original request body.

## Response Capture and MemoryEvent

For non-streaming responses, the plugin captures assistant content, status,
usage, finish reason, and tool-call presence from the configured paths. For
streaming SSE responses, it accumulates content across split chunks, supports
`\r\n`, `\r`, and `\n` line endings, handles `[DONE]`, and ignores role-only
chunks.

After the final response chunk, the plugin emits exactly one JSON
`MemoryEvent` with safe facts such as schema version, event id, idempotency key,
tenant, consumer, request id, path, digest, route, model, memory mode, policy
version, response status, stream flag, tool-call flag, usage, finish reason,
timestamps, plugin version, and no-store state.

Raw `user_content` and `assistant_content` are included only when response
capture is enabled, no-store is absent, parsing succeeded, the content is safe,
and the response status is eligible. Disabled capture, no-store, parse failure,
unsafe content, and ineligible status preserve safe facts while omitting raw
content.

## Fail-Open and Privacy

`ai-memory` is fail-open. Redis recent read failures, Console assemble failures,
request body replacement errors, response parsing errors, and Redis Stream
`XADD` dispatch failures do not change or block the user response.

The plugin does not log raw prompts, raw answers, credentials, authorization
headers, Redis passwords, or internal bearer tokens. Redis Stream dispatch
failure logs contain only safe diagnostics such as request id, stream name,
status, and fixed reason codes.

## Ordering and Isolation

Run `ai-memory` after identity and quota plugins so trusted tenant and consumer
identity are available. Run it before `ai-cache` and `ai-proxy` when memory
injection can change the model request.

## Ordering Recommendation

Recommended request-path order:

1. Identity or authentication plugins populate trusted tenant and consumer
   headers.
2. `ai-quota` runs with the trusted identity and can reject over-quota traffic
   before memory work starts.
3. `ai-memory` injects memory and records response facts.
4. `ai-cache` runs only with a conservative policy for memory-enabled routes:
   consumer-scoped cache, memory-policy or digest-aware keys, or bypass.
5. `ai-proxy` forwards the final memory-adjusted request to the provider.

Keep `ai-memory` isolated from `ai-cache`: memory events use `memory:events`,
recent state uses `memory:recent`, MemoryEvent payload schemas are separate
from cache payload schemas, and Console owns any memory vector namespace,
retention, deletion, management API, authorization, and repair policy.

## Cache and Memory Isolation

| Concern | `ai-memory` boundary |
| --- | --- |
| Redis Stream | Uses `memory:events`; never writes cache streams. |
| Redis key prefix | Reads Console materialized recent memory under `memory:recent`; never reads or writes cache key prefixes. |
| Vector namespace | Does not create, read, or mutate vector namespaces; Console owns memory vector indexing. |
| Payload schema | Emits `MemoryEvent`; never reuses cache request, response, replay, or settlement payload schemas. |
| Retention and deletion | Does not enforce retention or deletion; Console owns memory lifecycle policy. |
| Management APIs | Exposes no memory management APIs from the gateway. |
| Authorization checks | Trusts upstream identity headers and leaves memory authorization checks to Console APIs and workers. |

When `ai-memory` and `ai-cache` are both enabled, use a conservative cache
policy such as consumer-scoped cache, memory-policy or digest-aware cache keys,
or cache bypass until Console materializes memory-aware replay records.
