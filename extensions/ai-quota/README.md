---
title: AI 金额配额
keywords: [AI网关, AI配额, 金额配额]
description: ai-quota 金额余额准入插件配置参考
---

## 功能说明

`ai-quota` 在 AI 请求进入上游前读取 Console 派生的 Redis 热余额，余额大于等于 0 时放行，负余额按欠费拒绝，余额缺失按策略处理。插件只做请求准入，不在响应结束后解析 usage，不执行 Lua `EVAL`，也不扣减 Redis 余额。

插件不再提供 `/quota`、`/quota/refresh`、`/quota/delta` 等网关内管理接口，也不再支持 `admin_consumer`、`admin_path`、`redis_key_prefix`。账户、余额、价格、账单流水和 Redis 重建由 Console 或 billing-service 负责。

## Redis Key

- 余额默认 key：`billing:balance:{tenant}:{quota_scope}:{consumer}`
- 该 key 是 Console/billing-service 维护的派生缓存，网关插件只读取，不写入
- 历史价格和扣减相关字段仍可被配置解析接受，但不会触发任何响应期扣费或 Redis 写入

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis` | object | 无 | Redis 连接配置 |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | AI provider 标识，用于价格 key |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `balance_key_template` | string | `billing:balance:{tenant}:{quota_scope}:{consumer}` | 余额 key 模板 |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | 生效路径后缀 |
| `missing_balance_policy` | string | `deny` | 余额缺失策略：`deny` 或 `allow` |

`redis` 字段：

| 配置项 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `service_name` | string | 是 | - | Redis 服务名称 |
| `service_port` | int | 否 | static 服务为 80，其他为 6379 | Redis 端口 |
| `username` | string | 否 | - | Redis 用户名 |
| `password` | string | 否 | - | Redis 密码 |
| `timeout` | int | 否 | 1000 | 连接超时，单位毫秒 |
| `database` | int | 否 | 0 | Redis database |

## 配置示例

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
quota_scope: route:qwen
provider: dashscope
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
missing_balance_policy: deny
```

不同 AI 路由建议通过 Higress WasmPlugin `matchRules` 配置不同 `quota_scope`。`ai-quota` 只做余额准入；实际计费需要 `ai-billing` 携带由 `CONSOLE_INTERNAL_BILLING_TOKEN` 配置的 Bearer token 将事件投递到 Console 内部结算接口。
