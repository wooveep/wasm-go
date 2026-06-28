---
title: AI Memory
keywords: [AI网关, AI记忆, ai-memory]
description: ai-memory 薄网关插件配置参考
---

## 功能说明

`ai-memory` 是一个薄 Higress Wasm-Go 网关插件。它只处理请求路径上的
memory injection、响应捕获和事件投递：对支持的 JSON AI 请求执行 request
gating，读取租户和 consumer identity，从 Redis recent memory 读取 Console
物化的最近记忆，可选调用 `console_internal` 的
`POST /internal/memory/assemble`，替换请求体中的 messages，并在响应完成后向
Redis Stream `memory:events` 发送一个 `MemoryEvent`。

Console 负责持久化和后台记忆能力：PostgreSQL、recent-window materialization
（最近窗口物化）、daily digest、semantic recall、embedding、vector indexing、
deletion、repair、management API，以及 backend model cost attribution 都不属于
网关插件职责。

## 运行职责

插件负责：

- 按 `enable_path_suffixes` 和 JSON `content-type` 做 request gating。
- 从 `tenant_header`、`consumer_header`、`session_header` 和
  `request_id_header` 提取身份和请求事实。
- 使用 `recent_cache.key_prefix` 读取 Redis recent memory，默认前缀为
  `memory:recent`。
- 在 `digest` 或 `semantic` 模式下调用 Console assemble。
- 将 Console memory message 和安全 recent messages 注入 OpenAI-compatible
  request body。
- 捕获 assistant response、usage、finish reason、status code 和 tool-call 标记。
- 响应完成后通过 Redis Stream `XADD memory:events * event <MemoryEvent JSON>`
  投递 MemoryEvent。
- 对 Redis、Console、解析、替换、响应捕获和事件投递失败执行 fail-open。

插件不写 PostgreSQL，不生成摘要，不生成 embedding，不调用 vector database，不调用
provider model 做记忆工作，不执行 retention/deletion policy，不暴露管理 API，不做
backend model cost 归因或 billing settlement。

## 所有权边界

Console 负责所有持久化和重策略记忆职责：PostgreSQL 记录、recent-window
materialization、daily digest、semantic recall、embeddings、vector indexes、
retention、deletion、repair、management APIs、authorization checks 和 backend
model cost attribution。

网关插件不是记忆系统的事实来源。它不持久化会话，不生成记忆制品，不选择 retention
policy，不操作 vector storage，不修复缺失 turn，不暴露运维管理 API，也不做 billing
settlement。它只消费 Console 派生的运行时状态，并向 Console worker 投递一次
best-effort 事件。

## 隐私边界

`ai-memory` 不会记录 raw prompts、raw answers、credentials、Authorization
headers、Redis passwords 或 internal bearer tokens。Redis Stream 投递失败日志只包含
request id、stream、status 和固定 reason 等安全诊断信息。

当 `capture_response` 关闭、请求包含 no-store header、解析失败、内容不安全或响应状态不合格时，
MemoryEvent 仍保留安全事实，但省略 `user_content` 和 `assistant_content`。

## 配置说明

全局 `defaultConfig` 保存 Redis Stream、Redis recent、Console internal service、
identity headers、path suffixes 和 `fail_policy`。`matchRules[].config` 只保存路由
memory behavior，并继承全局外部目标。第一版不支持 route-level `redis_stream`、
`recent_cache` 或 `console_internal` 覆盖。

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

`memory_mode` 支持 `off`、`recent-only`、`digest` 和 `semantic`。`fail_policy`
当前只支持 `open`。

## 请求和响应流程

请求头阶段检查路径后缀、JSON content-type、租户和 consumer identity。缺少身份或路径不匹配时，
请求保持原样继续上游。

请求体阶段解析 OpenAI-compatible messages，提取最新 user content，生成 request digest，
读取 Redis recent memory，并在 `digest`/`semantic` 模式下调用 Console assemble。Console
可以返回 `inject`、`recent_only`、`skip` 或 `bypass` 决策。请求体替换失败时继续使用原始请求体。

响应阶段支持非流式和 streaming SSE response capture。插件处理 split chunks、`\r\n`、`\r`、
`\n`、`[DONE]` 和 role-only chunks，并在最终 chunk 后发送一个 MemoryEvent。

## 插件顺序和隔离

推荐将 `ai-memory` 放在 identity 和 `ai-quota` 之后，确保可信租户和 consumer identity 已经可用。
当 memory injection 会影响模型请求时，将 `ai-memory` 放在 `ai-cache` 和 `ai-proxy` 之前。

`ai-memory` 与 `ai-cache` 隔离：memory stream 使用 `memory:events`，recent key prefix 使用
`memory:recent`，payload schema 与 cache payload schema 分离。vector namespace、retention、
deletion、management APIs、authorization checks 和 repair policy 都由 Console 侧管理。
