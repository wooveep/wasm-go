---
title: AI Monetary Quota
keywords: [AI Gateway, AI Quota, Monetary Quota]
description: Configuration reference for monetary balance admission.
---

## Overview

`ai-quota` reads Console-derived Redis hot balance before forwarding enabled AI requests. Requests with a positive balance continue; missing or non-positive balances follow the configured policy. The plugin is admission-only: it does not parse response usage, does not run Lua `EVAL`, and does not deduct Redis balances after responses.

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
missing_balance_policy: deny
```

Default balance key: `billing:balance:{tenant}:{quota_scope}:{consumer}`.

The balance key is a Console/billing-service-owned derived cache. The gateway plugin reads it for admission and never treats it as a mutable ledger.

Legacy price and deduction-related fields can still be accepted by the parser for compatibility, but they do not trigger response-phase billing or Redis writes.

Use Higress WasmPlugin `matchRules` to bind different `quota_scope` values to different AI routes. `ai-quota` only gates requests; charging requires `ai-billing` delivery to Console's internal settlement endpoint with the Bearer token configured from `CONSOLE_INTERNAL_BILLING_TOKEN`.
