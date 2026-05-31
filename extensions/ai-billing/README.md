---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

## 功能说明

`ai-billing` 在 AI 响应完成后独立解析 token usage 和 model，构造请求级 billing event，并通过 HTTP callout 上报给 Console 内部 billing-service。HTTP callout 使用 `billing_service.auth_token` 生成 `Authorization: Bearer <token>` 鉴权头。插件默认 fail-open：billing-service 超时、网络失败或返回 5xx 时只记录日志，不阻塞用户响应。

`ai-billing` 不扣减 Redis 余额，也不直接更新账户或数据库。幂等、结算、账单流水、余额投影和补偿由 billing-service 负责。

## 事件字段

事件包含 `event_id`、`idempotency_key`、`request_id`、`consumer`、`route`、`provider`、`model`、`cluster`、`request_path`、`status_code`、`start_time_ms`、`end_time_ms`、`is_stream`、`usage`、`usage_missing` 和可选 `price_version`。

`route`、`provider`、`model` 使用 Console 原生对象事实格式：`{ "id"?: "...", "name"?: "..." }`。网关通常不知道 Console UUID，因此默认填充运行时 `name`。

## 配置示例

```yaml
billing_service:
  service_name: modelfusion-console.higress-system.svc.cluster.local
  service_port: 8080
  path: /internal/billing/events
  timeout: 750
  auth_token: <shared-secret>
quota_scope: route:qwen
provider: dashscope
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
fail_policy: open
```

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

`billing_service.path` 默认使用 `/internal/billing/events`，并通过 `billing_service.auth_token` 以 Bearer token 方式访问 Console 内部结算接口。Console 会使用 `CONSOLE_INTERNAL_BILLING_TOKEN` 校验该 token。

`ai-billing` 与 `ai-quota`、`ai-statistics` 相互独立，可以在另外两个插件关闭时单独上报 billing event；实际余额扣减和 Redis 投影刷新由 Console/billing-service 完成。
