---
title: AI 金额配额
keywords: [AI网关, AI配额, 金额配额]
description: ai-quota 金额余额准入与响应后扣费插件配置参考
---

## 功能说明

`ai-quota` 用 Redis 热余额做 AI 请求准入。请求命中 `enable_path_suffixes` 后，插件读取租户和 consumer 请求头，按 `balance_key_template` 构造余额 key；余额大于 0 时放行，余额缺失或非正余额按配置拒绝或放行。

响应完成后，插件独立调用 `pkg/tokenusage` 解析 token usage 和 model，按 `price_key_template` 构造输入、输出 token 价格 key，并通过 Redis Lua `EVAL` 读取价格、计算费用和扣减余额。

`ai-quota` 不再提供 `/quota`、`/quota/refresh`、`/quota/delta` 等管理接口。账户、余额、价格、账单流水和 Redis 重建由 Console 或 billing-service 负责。

## 配置示例

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  timeout: 1000
quota_scope: route:qwen
provider: dashscope
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
price_key_template: "billing:effective_price:{tenant}:{provider}:{model}:{token_type}"
amount_scale: 1000000
price_unit_tokens: 1000000
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
missing_balance_policy: deny
missing_price_policy: skip
missing_usage_policy: skip
```

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis` | object | 无 | Redis 连接配置 |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | AI provider 标识 |
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

费用计算：

```text
ceil(input_tokens * input_price / price_unit_tokens)
+ ceil(output_tokens * output_price / price_unit_tokens)
```

`ai-quota`、`ai-billing`、`ai-statistics` 互不依赖私有运行时状态，可以分别启用。
