---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

## 功能说明

`ai-billing` 在 AI 响应完成或流式请求终止后独立解析 token usage 和 model，构造请求级 billing event，并通过 HTTP callout 上报给 billing-service。HTTP callout 使用 `billing_service.auth_token` 生成 `Authorization: Bearer <token>` 鉴权头。插件默认 fail-open：billing-service 超时、网络失败或返回投递失败状态时只记录日志，不阻塞用户响应。

插件优先使用 provider 返回的 token usage，并在事件中标记 `usage_source=provider`。当 provider usage 缺失时，插件会对可结构化提取文本的 Chat Completions、Responses 和 Completions 请求使用 tokenizer 生成基础估算，并标记 `usage_source=estimated`；如果 provider usage 和本地估算都不可用，则上报零 token usage、`usage_missing=true` 和 `usage_source=missing`。

`ai-billing` 不计算最终权威费用，不应用租户折扣或覆盖价格，不扣减 Redis 余额，也不直接更新账户或数据库。幂等、结算、账单流水、余额投影和补偿由 billing-service 负责。

`ai-billing` 与 `ai-quota`、`ai-statistics` 相互独立，可以在另外两个插件关闭时单独上报 billing event。

## 运行属性

插件执行阶段：`默认阶段`
插件执行优先级：`270`

## Event 字段

事件包含以下字段：

| 字段 | 说明 |
| --- | --- |
| `event_id` | 插件在请求开始时生成的事件 ID |
| `idempotency_key` | 幂等键，默认与 `event_id` 相同 |
| `request_id` | 请求关联 ID，来自 `x-request-id` 或 Higress 请求属性 |
| `tenant` | 从 `tenant_header` 指定请求头读取的租户标识 |
| `consumer` | 从 `consumer_header` 指定请求头读取的 consumer 标识 |
| `quota_scope` | 当前请求匹配到的额度作用域，可来自全局配置或规则级覆盖 |
| `route` | Higress route 信息，通常为包含 `name` 的对象 |
| `provider` | AI provider 信息，通常为包含 `name` 的对象 |
| `model` | 响应中的 model 信息，缺失时使用未知模型标识，通常为包含 `name` 的对象 |
| `request_path` | 请求路径 |
| `status_code` | AI 响应状态码 |
| `usage` | 结构化 token usage，包含 `unit`、`input`、`output`、`total`，provider usage 可用时还会包含 `details.provider_usage`，并在 provider 暴露 cache 详情时包含 `input_cache_hit_tokens`、`input_cache_miss_tokens`、`output_tokens` |
| `usage_missing` | provider usage 和本地估算都不可用时为 `true` |
| `usage_source` | usage 来源，取值为 `provider`、`estimated` 或 `missing` |
| `start_time_ms` / `end_time_ms` | 请求开始和事件生成时间，毫秒时间戳 |
| `is_stream` | 是否流式响应 |
| `cluster` | 上游 cluster 名称 |
| `price_version` | 可选价格版本，来自 `x-ai-price-version` |

事件不会上报顶层 `input_tokens`、顶层 `output_tokens`、顶层 `total_tokens` 或 `gateway_calculated_cost`。`tenant`、`consumer`、`provider` 和 `quota_scope` 使用当前请求匹配到的配置解析。

Cache-aware token 字段只作为 `usage` 内的 usage facts 上报。为了兼容旧事件，插件会继续保留 `usage.input`、`usage.output` 和 `usage.total`。

## Usage 来源

| `usage_source` | 行为 |
| --- | --- |
| `provider` | 使用 provider 上报的 usage 作为权威值，`usage_missing=false`，并保留完整原始 usage 到 `usage.details.provider_usage` |
| `estimated` | provider usage 缺失且结构化文本可提取时，使用 tokenizer 估算 `usage.input`、`usage.output` 和 `usage.total`，不包含 cache-aware 字段或 `details.provider_usage` |
| `missing` | provider usage 缺失且无法安全估算时，上报零 token usage，`usage_missing=true` |

Provider usage 支持 OpenAI-compatible/GLM、Kimi、DeepSeek、Qwen、Claude/Anthropic 和 Gemini 常见 cache-aware 字段。派生 cache hit/miss 时会进行非负校验，并将 cached input token clamp 到 provider input token 范围内。

估算只覆盖可结构化提取文本的请求和响应：Chat Completions 的 `messages[].content`、Responses 的 `input` 和 `instructions`、Completions 的 `prompt`，以及对应的文本输出。未知、空或未映射模型默认使用 `o200k_base` tokenizer vocabulary，识别到的较旧 OpenAI-compatible 模型使用 `cl100k_base`。插件不会把完整原始 JSON 请求体当作 prompt 直接计数。

对于流式响应，正常 `endOfStream=true` 路径会投递一次 billing event；如果客户端中断或缺失最终 usage chunk，stream-done fallback 会在尚未投递时再尝试投递一次。流式估算只统计已经发送给客户端的文本 delta，避免把未下发内容计入输出 token。

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `billing_service` | object | 无 | billing-service HTTP callout 目标 |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | AI provider 标识 |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | 生效路径后缀 |
| `fail_policy` | string | `open` | 投递失败策略，当前支持 `open` |

全局 `defaultConfig` 必须配置完整的 `billing_service`，并推荐放置共享的租户、consumer、路径后缀和 fail-open 设置。`matchRules[].config` 可以只配置当前路由差异，例如 `provider`、`quota_scope`、`enable_path_suffixes` 或 `fail_policy`；未配置字段会继承全局值。为了兼容旧配置，规则级 `billing_service` 仍可配置，并会为该规则创建独立 HTTP callout 客户端。

`billing_service` 字段：

| 配置项 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `service_name` | string | 是 | `modelfusion-console.higress-system.svc.cluster.local` | billing-service 服务名称 |
| `service_port` | int | 否 | 80 | billing-service 服务端口 |
| `path` | string | 否 | `/internal/billing/events` | billing event 上报路径 |
| `timeout` | int | 否 | 500 | HTTP callout 超时时间，单位毫秒 |
| `auth_token` | string | 否 | 无 | billing-service 共享鉴权密钥，用于生成 `Authorization: Bearer <token>`；示例必须使用 `<shared-secret>` 占位符 |

## 配置示例

```yaml
defaultConfig:
  billing_service:
    service_name: modelfusion-console.higress-system.svc.cluster.local
    service_port: 8080
    path: /internal/billing/events
    timeout: 750
    auth_token: <shared-secret>
  quota_scope: global
  provider: default
  tenant_header: x-mse-tenant
  consumer_header: x-mse-consumer
  enable_path_suffixes:
    - /v1/chat/completions
    - /v1/messages
  fail_policy: open
matchRules:
  - ingress:
      - qwen
    config:
      quota_scope: route:qwen
      provider: dashscope
```
