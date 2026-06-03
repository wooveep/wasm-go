## Why

`ai-billing` currently parses both `defaultConfig` and `matchRules[].config` as complete plugin configs, so route-level overrides must repeat `billing_service` and every required field. This prevents operators from defining shared billing-service and identity defaults globally while only overriding provider or route-specific behavior per ingress/route.

## What Changes

- Switch `ai-billing` to an override parsing model where the global config is parsed as a complete `BillingConfig` and each matched rule config inherits it before applying explicit overrides.
- Keep `billing_service` required in global config, while allowing rule configs to omit it and inherit the global billing-service client.
- Allow rule-level overrides for `provider`, `quota_scope`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`.
- Preserve backward compatibility for existing match rules that provide full `billing_service` and complete rule config.
- Ensure `provider` and `quota_scope` semantics are reflected in emitted billing events, including route-specific provider selection.
- Update `extensions/ai-billing/plugin.yaml` and resource documentation/schema examples to show the recommended shape: shared settings in `defaultConfig`, only differences in `matchRules[].config`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ai-billing-events`: Change `ai-billing` configuration requirements and billing event semantics to support global defaults with rule-level partial overrides.
- `ai-plugin-resource-docs`: Update `ai-billing` resource schema and examples to document the inherited default config model and recommended match-rule override shape.

## Impact

- Affected code: `extensions/ai-billing/main.go`, `extensions/ai-billing/main_test.go`, and plugin/resource YAML and README files for `ai-billing`.
- Affected runtime behavior: plugin initialization, match-rule config resolution, billing event provider/quota-scope fields, fail-policy and path-suffix handling.
- Compatibility: Existing full rule configs remain valid. Configurations without global `billing_service` continue to fail initialization with a clear error.
- Dependencies: Uses existing wrapper override parsing support; no new external dependencies are expected.
