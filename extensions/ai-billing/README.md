---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

## 功能说明

`ai-billing` 在 AI 响应完成后独立解析 token usage 和 model，构造请求级 billing event，并通过 Redis Stream 写入 Console/billing-service 的结算队列。插件使用 Redis `XADD <stream> * event <BillingEvent JSON>`，默认写入 `billing:events`。插件默认 fail-open：Redis 投递超时、网络失败或返回错误时只记录日志，不阻塞用户响应。

`ai-billing` 不计算最终权威费用，不应用租户折扣或覆盖价格，不扣减 Redis 余额，也不直接更新账户或数据库。幂等、结算、账单流水、余额投影和补偿由 Console/billing-service 负责。

## 事件字段

事件包含 `event_id`、`idempotency_key`、`request_id`、`tenant`、`consumer`、`quota_scope`、`route`、`provider`、`model`、`cluster`、`request_path`、`status_code`、`start_time_ms`、`end_time_ms`、`is_stream`、`usage`、`usage_missing` 和可选 `price_version`。

当 provider usage 暴露 cache 详情时，`usage` 还会包含 `input_cache_hit_tokens`、`input_cache_miss_tokens` 和 `output_tokens`。为了兼容旧事件，`usage.input`、`usage.output` 和 `usage.total` 会继续保留。

`route`、`provider`、`model` 使用 Console 原生对象事实格式：`{ "id"?: "...", "name"?: "..." }`。网关通常不知道 Console UUID，因此默认填充运行时 `name`。

## 配置示例

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.higress-system.svc.cluster.local
    service_port: 6379
    database: 0
    timeout: 500
    stream: billing:events
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

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis_stream` | object | 无 | Redis Stream 投递目标 |
| `redis_stream.service_name` | string | 无 | Redis 服务名或 Console 管理的服务注册名 |
| `redis_stream.service_port` | integer | `6379` | Redis 服务端口 |
| `redis_stream.username` | string | 空 | Redis 用户名 |
| `redis_stream.password` | string | 空 | Redis 密码，需按敏感字段处理 |
| `redis_stream.database` | integer | `0` | Redis database |
| `redis_stream.timeout` | integer | `500` | Redis callout 超时时间，单位毫秒 |
| `redis_stream.stream` | string | `billing:events` | Redis Stream 名称 |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | AI provider 标识 |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | 生效路径后缀 |
| `fail_policy` | string | `open` | 投递失败策略，当前支持 `open` |

全局 `defaultConfig` 必须配置 `redis_stream.service_name`，并推荐放置共享的租户、consumer、路径后缀和 fail-open 设置。`matchRules[].config` 可以只配置当前路由差异，例如 `provider`、`quota_scope`、`enable_path_suffixes` 或 `fail_policy`；未配置字段会继承全局值。规则级 `redis_stream` 不支持配置，Redis Stream 目标属于插件实例共享配置。

Console 管理的 `ai-billing` 配置会把 Redis 服务投影为 McpBridge registry 名称，例如 `redis-stack-server.dns`。旧的 `billing_service`、`path` 和 `auth_token` 配置已移除；从旧版本升级时需要删除并重新创建已有 HTTP 形态的 `WasmPlugin`，或通过 Console 重新投影托管绑定。

`ai-billing` 与 `ai-quota`、`ai-statistics` 相互独立，可以在另外两个插件关闭时单独上报 billing event；实际余额扣减和 Redis 投影刷新由 Console/billing-service 完成。
