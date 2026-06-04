---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

## 功能说明

`ai-billing` 在 AI 响应完成后独立解析 token usage 和 model，构造请求级 billing event，并通过 HTTP callout 上报给 billing-service。HTTP callout 使用 `billing_service.auth_token` 生成 `Authorization: Bearer <token>` 鉴权头。插件默认 fail-open：billing-service 超时、网络失败或返回投递失败状态时只记录日志，不阻塞用户响应。

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
| `route` | Higress route 名称 |
| `provider` | AI provider 标识 |
| `model` | 响应中的 model，缺失时使用未知模型标识 |
| `request_path` | 请求路径 |
| `status_code` | AI 响应状态码 |
| `usage` | 结构化 token usage，包含 `unit`、`input`、`output`、`total`、`details`，并在 provider usage 暴露 cache 详情时包含 `input_cache_hit_tokens`、`input_cache_miss_tokens`、`output_tokens` |
| `usage_missing` | 是否未解析到可用 token usage |
| `start_time_ms` / `end_time_ms` | 请求开始和事件生成时间，毫秒时间戳 |
| `is_stream` | 是否流式响应 |
| `cluster` | 上游 cluster 名称 |
| `price_version` | 可选价格版本，来自 `x-ai-price-version` |

事件不会上报顶层 `input_tokens`、顶层 `output_tokens`、顶层 `total_tokens` 或 `gateway_calculated_cost`。`tenant`、`consumer`、`provider` 和 `quota_scope` 使用当前请求匹配到的配置解析。

Cache-aware token 字段只作为 `usage` 内的 usage facts 上报。为了兼容旧事件，插件会继续保留 `usage.input`、`usage.output` 和 `usage.total`。

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
