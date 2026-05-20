---
title: AI Billing Events
keywords: [AI Gateway, AI Billing, ai-billing]
description: Configuration reference for request-level AI billing event delivery.
---

## Overview

`ai-billing` parses token usage and model independently after an enabled AI response completes, builds a request-level billing event, and sends it to billing-service through an HTTP callout. Delivery is fail-open by default: timeouts, network failures, and 5xx responses are logged but do not block the user response.

The plugin does not deduct Redis balances or update account databases. Idempotency, settlement, statements, balance projection, and reconciliation belong to billing-service.

## Example

```yaml
billing_service:
  service_name: modelfusion-console.higress-system.svc.cluster.local
  service_port: 8080
  path: /internal/billing/events
  timeout: 750
quota_scope: route:qwen
provider: dashscope
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
fail_policy: open
```

Events include request identity, tenant, consumer, quota scope, provider, model, route, cluster, request path, status code, timing, stream flag, token counts, `usage_missing`, optional price version, and optional gateway-calculated cost.

`ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently.
