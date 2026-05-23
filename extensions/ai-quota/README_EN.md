---
title: AI Monetary Quota
keywords: [AI Gateway, AI Quota, Monetary Quota]
description: Configuration reference for monetary balance admission and post-response deduction.
---

## Overview

`ai-quota` checks a Redis hot balance before forwarding enabled AI requests. Requests with a positive balance continue; missing or non-positive balances follow the configured policy. After the response completes, the plugin parses token usage and model with `pkg/tokenusage`, reads tenant effective prices from Redis, then uses one Lua `EVAL` call to calculate and deduct the monetary cost.

The plugin no longer exposes gateway-hosted quota management APIs such as `/quota`, `/quota/refresh`, or `/quota/delta`. Account balances, prices, billing statements, and Redis rebuilds are owned by Console or billing-service.

## Configuration

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
amount_scale: 1000000
price_unit_tokens: 1000000
missing_balance_policy: deny
missing_price_policy: skip
missing_usage_policy: skip
```

Default balance key: `billing:balance:{tenant}:{quota_scope}:{consumer}`.

Default price key: `billing:effective_price:{tenant}:{provider}:{model}:{token_type}`, where `token_type` is `input` or `output`.

Cost is calculated as:

```text
ceil(input_tokens * input_price / price_unit_tokens)
+ ceil(output_tokens * output_price / price_unit_tokens)
```

Use Higress WasmPlugin `matchRules` to bind different `quota_scope` and `provider` values to different AI routes. `ai-quota`, `ai-billing`, and `ai-statistics` are independently deployable.
