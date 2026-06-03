## Context

The latest `extensions/ai-billing` implementation emits a Console-oriented billing event from response completion paths. The implemented payload is defined by `BillingEvent` and verified by `extensions/ai-billing/main_test.go`: events include `event_id`, `idempotency_key`, request correlation, `consumer`, route/provider/model/path/status metadata, structured `usage`, `usage_missing`, timing, stream state, cluster, and optional `price_version`.

The resource package currently describes older fields that are not emitted by the implementation. Tests explicitly assert that `tenant`, `quota_scope`, flat token fields, and `gateway_calculated_cost` are not present in the serialized event. Resource docs and catalog schema must therefore follow the implementation and extension examples instead of the older conceptual event shape.

## Goals / Non-Goals

**Goals:**

- Make `resources/plugins/ai-billing/README.md` and `README_EN.md` describe the implemented event payload and operational behavior.
- Make `resources/plugins/ai-billing/spec.yaml` expose the implemented configuration fields, including `billing_service.auth_token`, and keep examples aligned with `extensions/ai-billing/plugin.yaml`.
- Preserve compatibility fields that the parser still accepts, such as `quota_scope` and `tenant_header`, while making clear that they are configuration metadata and not emitted event fields.
- Keep all examples safe by using `<shared-secret>` for `auth_token`.

**Non-Goals:**

- Change `extensions/ai-billing` Go code or tests.
- Add billing-service ingestion, settlement, pricing, compensation, or database behavior.
- Publish real secrets, tenant identifiers, API keys, or internal user identifiers in documentation examples.

## Decisions

1. Treat `extensions/ai-billing/main.go` and its tests as the source of truth.

   The resource docs will mirror the implemented `BillingEvent` struct and test assertions, not stale text in older docs. This avoids documenting fields that operators cannot observe in delivered events.

2. Keep configuration fields separate from event fields.

   `quota_scope` and `tenant_header` remain in resource schemas because `parseConfig` still accepts them and extension examples still show `quota_scope`. The event field list will not imply that `quota_scope` or `tenant` are serialized in billing events.

3. Document structured usage rather than flat token counters.

   Current events use `usage.unit`, `usage.input`, `usage.output`, `usage.total`, and `usage.details`. Resource docs will remove `input_tokens`, `output_tokens`, and `total_tokens` as top-level event fields.

4. Align examples with extension defaults and placeholders.

   The resource package will keep the representative Console service endpoint from `extensions/ai-billing/plugin.yaml` and use `auth_token: <shared-secret>`. The docs will describe Bearer authorization without exposing or inventing a concrete secret.

## Risks / Trade-offs

- Stale upstream event spec text could reintroduce removed fields -> update the `ai-billing-events` OpenSpec delta alongside the resource-docs delta.
- Operators may confuse accepted config fields with emitted event fields -> separate configuration tables from event payload tables and explicitly state non-emitted fields where useful.
- Console ingestion may still depend on older conceptual fields -> keep this change scoped to resource docs and schema; implementation behavior already excludes those fields and tests assert that contract.
- Example drift with extension plugin YAML can recur -> include verification tasks that compare resource examples with `extensions/ai-billing/plugin.yaml` before completion.
