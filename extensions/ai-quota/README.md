---
title: AI 金额配额
keywords: [AI网关, AI配额, 金额配额]
description: ai-quota 金额余额准入与响应后扣费插件配置参考
---

## 功能说明

`ai-quota` 在 AI 请求进入上游前读取 Redis 热余额，余额大于 0 时放行，余额缺失或非正余额按策略处理。响应结束后，插件通过 `pkg/tokenusage` 独立解析 token usage 和 model，读取 Redis 中租户生效价格，并用 Lua `EVAL` 原子计算费用和扣减余额。

插件不再提供 `/quota`、`/quota/refresh`、`/quota/delta` 等网关内管理接口，也不再支持 `admin_consumer`、`admin_path`、`redis_key_prefix`。账户、余额、价格、账单流水和 Redis 重建由 Console 或 billing-service 负责。

## Redis Key

- 余额默认 key：`billing:balance:{tenant}:{quota_scope}:{consumer}`
- 价格默认 key：`billing:effective_price:{tenant}:{provider}:{model}:{token_type}`
- `token_type` 为 `input` 或 `output`
- 金额和价格均为整数，默认按 `amount_scale: 1000000`、`price_unit_tokens: 1000000` 表示

费用计算：

```text
ceil(input_tokens * input_price / price_unit_tokens)
+ ceil(output_tokens * output_price / price_unit_tokens)
```

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis` | object | 无 | Redis 连接配置 |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | AI provider 标识，用于价格 key |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `balance_key_template` | string | `billing:balance:{tenant}:{quota_scope}:{consumer}` | 余额 key 模板 |
| `price_key_template` | string | `billing:effective_price:{tenant}:{provider}:{model}:{token_type}` | 价格 key 模板 |
| `amount_scale` | int | `1000000` | 金额缩放比例 |
| `price_unit_tokens` | int | `1000000` | 价格单位 token 数 |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | 生效路径后缀 |
| `missing_balance_policy` | string | `deny` | 余额缺失策略：`deny` 或 `allow` |
| `missing_price_policy` | string | `skip` | 价格缺失时跳过扣减 |
| `missing_usage_policy` | string | `skip` | usage 缺失时跳过扣减 |

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
price_key_template: "billing:effective_price:{tenant}:{provider}:{model}:{token_type}"
missing_balance_policy: deny
missing_price_policy: skip
missing_usage_policy: skip
```

不同 AI 路由建议通过 Higress WasmPlugin `matchRules` 配置不同 `quota_scope` 和 `provider`。`ai-quota` 与 `ai-statistics`、`ai-billing` 相互独立，可以单独启用。
