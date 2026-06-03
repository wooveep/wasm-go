## 1. Pre-Change Analysis

- [x] 1.1 Re-read `extensions/ai-billing/main.go`, `extensions/ai-billing/main_test.go`, and `extensions/ai-billing/plugin.yaml` to confirm the implemented event payload, configuration parser, and example defaults.
- [x] 1.2 Run GitNexus impact analysis for any code symbol that must be edited; if implementation remains resource-only, record that no Go symbols are edited.
- [x] 1.3 Compare `resources/plugins/ai-billing` with the extension implementation and list stale event fields, missing auth-token guidance, and example drift.

## 2. Resource Documentation

- [x] 2.1 Update `resources/plugins/ai-billing/README.md` to document the implemented event fields, structured `usage` object, idempotency/event identity behavior, Bearer callout authorization, and fail-open delivery semantics.
- [x] 2.2 Update `resources/plugins/ai-billing/README_EN.md` with equivalent English content.
- [x] 2.3 Remove or correct any resource documentation that says events emit `tenant`, `quota_scope`, top-level token count fields, or `gateway_calculated_cost`.
- [x] 2.4 Keep configuration documentation for accepted parser fields, including `quota_scope`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, `fail_policy`, and all `billing_service` fields.

## 3. Resource Schema

- [x] 3.1 Update `resources/plugins/ai-billing/spec.yaml` descriptions and examples to include `billing_service.auth_token: <shared-secret>` and the callout target from `extensions/ai-billing/plugin.yaml`.
- [x] 3.2 Ensure global and route config schemas expose the implemented configuration fields and keep `fail_policy` restricted to `open`.
- [x] 3.3 Ensure examples use only placeholder secret values and preserve representative `quota_scope`, `provider`, tenant header, consumer header, and path suffix values.

## 4. Verification

- [x] 4.1 Run YAML validation or an equivalent parser check for `resources/plugins/ai-billing/spec.yaml`.
- [x] 4.2 Search `resources/plugins/ai-billing` to confirm stale emitted-event fields are not documented as serialized event fields.
- [x] 4.3 Run `openspec status --change update-ai-billing-resources` and confirm all artifacts are complete.
- [x] 4.4 Run `graphify update .` after modifying resource files.
- [x] 4.5 Run `gitnexus_detect_changes()` before committing to confirm the affected scope matches the expected resource documentation/schema change.
