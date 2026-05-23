---
title: AI Billing Events
keywords: [AI Gateway, AI Billing, ai-billing]
description: Configuration reference for request-level AI billing event delivery.
---

## Overview

`ai-billing` parses token usage and model independently after an enabled AI response completes, builds a request-level billing event, and sends it to billing-service through an HTTP callout. Delivery is fail-open by default: timeouts, network failures, and 5xx responses are logged but do not block the user response.

The plugin does not deduct Redis balances or update account databases. Idempotency, settlement, statements, balance projection, and reconciliation belong to billing-service.

`ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently.

## Runtime Properties

Plugin execution phase: `default phase`
Plugin execution priority: `270`

## Event Fields

Events include `request_id`, `idempotency_key`, `tenant`, `consumer`, `quota_scope`, `provider`, `model`, `route`, `cluster`, `request_path`, `status_code`, `start_time_ms`, `end_time_ms`, `is_stream`, token counts, `usage_missing`, optional `price_version`, and optional `gateway_calculated_cost`.

## Configuration

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `billing_service` | object | none | billing-service HTTP callout target |
| `quota_scope` | string | `global` | Quota scope for the current route or rule |
| `provider` | string | `default` | AI provider identifier |
| `tenant_header` | string | `x-mse-tenant` | Request header containing tenant identity |
| `consumer_header` | string | `x-mse-consumer` | Request header containing consumer identity |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | Enabled AI path suffixes |
| `fail_policy` | string | `open` | Delivery failure policy. Currently supports `open` |

`billing_service` fields:

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `service_name` | string | yes | `modelfusion-console.higress-system.svc.cluster.local` | billing-service service name |
| `service_port` | int | no | 80 | billing-service service port |
| `path` | string | no | `/billing/events` | Billing event delivery path |
| `timeout` | int | no | 500 | HTTP callout timeout in milliseconds |
| `auth_token` | string | no | none | Shared secret for billing-service authorization; examples must use the `<shared-secret>` placeholder |

## Example

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
