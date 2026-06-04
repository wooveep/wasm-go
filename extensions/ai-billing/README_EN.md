---
title: AI Billing Events
keywords: [AI Gateway, AI Billing, ai-billing]
description: Configuration reference for request-level AI billing event delivery.
---

## Overview

`ai-billing` parses token usage and model independently after an enabled AI response completes, builds a request-level billing event, and sends it to Console's internal billing-service through an HTTP callout. The callout uses `billing_service.auth_token` to send `Authorization: Bearer <token>`. Delivery is fail-open by default: timeouts, network failures, and 5xx responses are logged but do not block the user response.

The plugin does not calculate final authoritative costs, apply tenant discounts, apply override prices, deduct Redis balances, or update account databases. Idempotency, settlement, statements, balance projection, and reconciliation belong to billing-service.

## Example

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

Events include `event_id`, `idempotency_key`, `request_id`, `tenant`, `consumer`, `quota_scope`, `route`, `provider`, `model`, `cluster`, request path, status code, timing, stream flag, `usage`, `usage_missing`, and optional price version.

When provider usage exposes cache details, `usage` also includes `input_cache_hit_tokens`, `input_cache_miss_tokens`, and `output_tokens`. Legacy `usage.input`, `usage.output`, and `usage.total` stay present for compatibility.

`route`, `provider`, and `model` use Console-native object facts: `{ "id"?: "...", "name"?: "..." }`. Gateways usually do not know Console UUIDs, so the plugin populates runtime `name` values by default.

The global `defaultConfig` must contain the complete `billing_service`, and should hold shared tenant, consumer, path suffix, and fail-open defaults. `matchRules[].config` can contain only route-specific differences such as `provider`, `quota_scope`, `enable_path_suffixes`, or `fail_policy`; omitted fields inherit from the global config. For backward compatibility, a rule-level `billing_service` is still accepted and creates a rule-specific HTTP callout client.

`billing_service.path` defaults to `/internal/billing/events` and uses the configured Bearer token for Console internal settlement authentication. Console validates that token against `CONSOLE_INTERNAL_BILLING_TOKEN`.

`ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently. Balance debit and Redis projection refresh remain owned by Console/billing-service.
