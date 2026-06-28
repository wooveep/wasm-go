---
title: AI Memory
keywords: [AI Gateway, AI Memory, ai-memory]
description: Configuration reference for the ai-memory thin gateway plugin.
---

## Overview

`ai-memory` is a thin Higress Wasm-Go gateway plugin. It handles request-time
memory injection, response capture, and event delivery for OpenAI-compatible
JSON AI traffic. It performs request gating, reads tenant and consumer identity,
loads Console-materialized Redis recent memory, optionally calls
`POST /internal/memory/assemble`, rewrites the request body with safe memory
messages, and emits one `MemoryEvent` to Redis Stream `memory:events` field
`event` after the response completes.

Console owns durable memory responsibilities: PostgreSQL persistence,
recent-window materialization, daily digest generation, semantic recall,
embedding, vector indexing, deletion, repair, management APIs, and backend model
cost attribution are outside the gateway plugin.

## Runtime Responsibilities

The plugin:

- Runs request gating by `enable_path_suffixes` and JSON `content-type`.
- Extracts tenant identity, consumer identity, optional session, and request id
  through `tenant_header`, `consumer_header`, `session_header`, and
  `request_id_header`.
- Reads Redis recent memory with `recent_cache.key_prefix`, defaulting to
  `memory:recent`.
- Calls Console assemble only for `digest` and `semantic` memory modes.
- Injects a Console memory message and safe recent messages into the
  OpenAI-compatible request body.
- Captures assistant response facts, usage, finish reason, status code, and
  tool-call markers.
- Sends `XADD memory:events * event <MemoryEvent JSON>` after response
  completion.
- Uses fail-open behavior for Redis, Console, parsing, replacement, response
  capture, and event dispatch failures.

The plugin does not write PostgreSQL, generate digests, generate embeddings,
call vector databases, call provider models for memory work, apply retention or
deletion policy, expose management APIs, calculate backend model cost, or create
billing settlement records.

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

## Privacy Boundary

`ai-memory` does not log raw prompts, raw answers, credentials, Authorization
headers, Redis passwords, or internal bearer tokens. Redis Stream dispatch
failure logs include only safe diagnostics such as request id, stream, status,
and fixed reason codes.

When `capture_response` is false, a no-store header is present, parsing fails,
content is unsafe, or the status code is ineligible, the MemoryEvent keeps safe
facts but omits `user_content` and `assistant_content`.

## Configuration

Global `defaultConfig` contains Redis Stream, Redis recent memory, Console
internal service, identity headers, path suffixes, and `fail_policy`.
`matchRules[].config` contains route memory behavior and inherits the global
external targets. The first version rejects route-level `redis_stream`,
`recent_cache`, and `console_internal` overrides.

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.higress-system.svc.cluster.local
    service_port: 6379
    username: ""
    password: <redis-password>
    database: 0
    timeout: 500
    stream: memory:events
  recent_cache:
    service_name: redis-stack-server.higress-system.svc.cluster.local
    service_port: 6379
    username: ""
    password: <redis-password>
    database: 0
    timeout: 50
    key_prefix: memory:recent
  console_internal:
    service_name: modelfusion-console-api.higress-system.svc.cluster.local
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

`memory_mode` accepts `off`, `recent-only`, `digest`, and `semantic`.
`fail_policy` currently supports only `open`.

## Request and Response Flow

The header phase checks path suffix, JSON content type, tenant, and consumer
identity. Missing identity or unsupported paths continue upstream unchanged.

The body phase parses OpenAI-compatible messages, extracts the latest user
content, builds a request digest, loads Redis recent memory, and optionally
calls Console assemble for `digest` and `semantic` modes. Console may return
`inject`, `recent_only`, `skip`, or `bypass`. Request body replacement failures
continue upstream with the original body.

The response phase supports non-streaming and streaming SSE response capture.
It handles split chunks, `\r\n`, `\r`, `\n`, `[DONE]`, and role-only chunks, and
sends exactly one MemoryEvent after the final chunk.

## Ordering and Isolation

Run `ai-memory` after identity plugins and `ai-quota` so trusted tenant and
consumer identity are available. Run it before `ai-cache` and `ai-proxy` when
memory injection can affect the model request.

`ai-memory` is isolated from `ai-cache`: memory events use `memory:events`,
recent keys use `memory:recent`, and MemoryEvent payload schemas are separate
from cache payload schemas. Vector namespaces, retention, deletion, management
APIs, authorization checks, and repair policy belong to Console.

## Cache and Memory Isolation

| Concern | `ai-memory` boundary |
| --- | --- |
| Redis Stream | Uses `memory:events`; never writes cache streams. |
| Redis key prefix | Reads Console-materialized recent memory under `memory:recent`; never reads or writes cache key prefixes. |
| Vector namespace | Does not create, read, or mutate vector namespaces; Console owns memory vector indexing. |
| Payload schema | Emits `MemoryEvent`; never reuses cache request, response, replay, or settlement payload schemas. |
| Retention and deletion | Does not enforce retention or deletion; Console owns memory lifecycle policy. |
| Management APIs | Exposes no memory management APIs from the gateway. |
| Authorization checks | Trusts upstream identity headers and leaves memory authorization checks to Console APIs and workers. |
