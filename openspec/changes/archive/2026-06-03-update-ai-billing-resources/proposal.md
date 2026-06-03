## Why

`resources/plugins/ai-billing` no longer fully reflects the latest `extensions/ai-billing` implementation. The resource README files and catalog schema still describe outdated billing event fields and need to match the current plugin payload, callout authentication, fail-open behavior, and examples before the resource package is published or consumed by Console.

## What Changes

- Update `resources/plugins/ai-billing/README.md` and `README_EN.md` so the documented event contract matches `BillingEvent` in `extensions/ai-billing/main.go`.
- Remove resource documentation claims that emitted events contain `tenant`, `quota_scope`, flat token count fields, or `gateway_calculated_cost`; current implementation explicitly excludes those fields.
- Document the current structured `usage` object, `event_id`, `idempotency_key`, `consumer`, `provider`, route/cluster metadata, `price_version`, and `usage_missing` behavior.
- Align resource examples with `extensions/ai-billing/plugin.yaml`, including `billing_service.auth_token: <shared-secret>`, HTTP callout target fields, path suffix defaults, route-level provider metadata, and `fail_policy: open`.
- Update `resources/plugins/ai-billing/spec.yaml` descriptions, schema, examples, and route config where needed so catalog UI users see the current supported configuration.
- Refresh the OpenSpec billing event/resource requirements to describe the implemented payload and resource documentation expectations.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ai-plugin-resource-docs`: require the `ai-billing` resource package to document the current implemented event payload, auth token placeholder guidance, and examples.
- `ai-billing-events`: correct the billing event contract to match the latest plugin implementation and tests.

## Impact

- Affected resource files: `resources/plugins/ai-billing/README.md`, `resources/plugins/ai-billing/README_EN.md`, and `resources/plugins/ai-billing/spec.yaml`.
- Affected OpenSpec contracts: `ai-plugin-resource-docs` and `ai-billing-events`.
- No Go implementation change is intended; the resource package follows existing `extensions/ai-billing` behavior.
- GitNexus pre-edit impact for `resources/plugins/ai-billing/spec.yaml`: LOW risk, 0 direct callers, 0 affected execution flows, 0 affected modules.
