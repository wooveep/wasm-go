---
title: AI Monetary Quota
keywords: [AI Gateway, AI Quota, Monetary Quota]
description: Configuration reference for monetary balance admission and post-response deduction.
---

## Overview

`ai-quota` checks a Redis hot balance before forwarding enabled AI requests. Requests with a positive balance continue; missing or non-positive balances follow the configured policy. After the response completes, the plugin parses token usage and model with `pkg/tokenusage`, reads tenant effective prices from Redis, then uses one Lua `EVAL` call to calculate and deduct the monetary cost.

The plugin no longer owns in-gateway quota management. Account balances, prices, billing statements, Redis rebuilds, idempotency, and reconciliation are owned by Console or billing-service.

## Runtime Properties

Plugin execution phase: `default phase`
Plugin execution priority: `280`

## Redis Keys

- Default balance key: `billing:balance:{tenant}:{quota_scope}:{consumer}`
- Default price key: `billing:effective_price:{tenant}:{provider}:{model}:{token_type}`
- `token_type` is `input` or `output`
- Amounts and prices are integers, represented by default with `amount_scale: 1000000` and `price_unit_tokens: 1000000`

Cost is calculated as:

```text
ceil(input_tokens * input_price / price_unit_tokens)
+ ceil(output_tokens * output_price / price_unit_tokens)
```

## Configuration

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `redis` | object | none | Redis connection configuration |
| `quota_scope` | string | `global` | Quota scope for the current route or rule |
| `provider` | string | `default` | AI provider identifier used in price keys |
| `tenant_header` | string | `x-mse-tenant` | Request header containing tenant identity |
| `consumer_header` | string | `x-mse-consumer` | Request header containing consumer identity |
| `balance_key_template` | string | `billing:balance:{tenant}:{quota_scope}:{consumer}` | Balance key template |
| `price_key_template` | string | `billing:effective_price:{tenant}:{provider}:{model}:{token_type}` | Price key template |
| `amount_scale` | int | `1000000` | Monetary amount scale |
| `price_unit_tokens` | int | `1000000` | Token count represented by one price unit |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | Enabled AI path suffixes |
| `missing_balance_policy` | string | `deny` | Missing balance policy: `deny` or `allow` |
| `missing_price_policy` | string | `skip` | Skip deduction when prices are missing |
| `missing_usage_policy` | string | `skip` | Skip deduction when usage is missing |

`redis` fields:

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `service_name` | string | yes | - | Redis service name |
| `service_port` | int | no | 80 for static services, otherwise 6379 | Redis service port |
| `username` | string | no | - | Redis username |
| `password` | string | no | - | Redis password |
| `timeout` | int | no | 1000 | Connection timeout in milliseconds |
| `database` | int | no | 0 | Redis database |

## Example

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  timeout: 1000
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

Use Higress WasmPlugin `matchRules` to bind different `quota_scope` and `provider` values to different AI routes. `ai-quota`, `ai-billing`, and `ai-statistics` can be deployed independently.
