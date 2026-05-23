## Why

The plugin resource catalog is out of sync with the current AI plugin behavior. `resources/plugins/ai-quota` still describes the removed token-quota admin APIs, and `ai-billing` has extension documentation but no generated resource catalog package.

This change makes the resource-facing documentation and `spec.yaml` files match the implemented `ai-quota` monetary quota model and the new `ai-billing` event reporting plugin.

## What Changes

- Update `resources/plugins/ai-quota/README.md`, `README_EN.md`, and `spec.yaml` to describe monetary balance admission, post-response deduction, Redis hot balance and effective price keys, route-level `quota_scope`/`provider`, and missing-data policies.
- Remove obsolete `ai-quota` resource documentation for `redis_key_prefix`, `admin_consumer`, `admin_path`, and gateway-hosted `/quota`, `/quota/refresh`, and `/quota/delta` management APIs.
- Generate a new `resources/plugins/ai-billing` resource package with Chinese and English README files plus `spec.yaml`.
- Ensure `ai-billing` resource docs describe request-level billing event generation, billing-service HTTP callout configuration, fail-open delivery, event fields, and independence from `ai-quota` and `ai-statistics`.
- Align resource `spec.yaml` examples and schemas with the current extension `plugin.yaml` examples and README files for both plugins.

## Capabilities

### New Capabilities

- `ai-plugin-resource-docs`: Resource catalog documentation and `spec.yaml` packaging for the `ai-quota` and `ai-billing` AI gateway plugins.

### Modified Capabilities

None.

## Impact

- Affected resource files:
  - `resources/plugins/ai-quota/README.md`
  - `resources/plugins/ai-quota/README_EN.md`
  - `resources/plugins/ai-quota/spec.yaml`
  - `resources/plugins/ai-billing/README.md`
  - `resources/plugins/ai-billing/README_EN.md`
  - `resources/plugins/ai-billing/spec.yaml`
- Reference sources:
  - `extensions/ai-quota/README.md`
  - `extensions/ai-quota/README_EN.md`
  - `extensions/ai-quota/plugin.yaml`
  - `extensions/ai-billing/README.md`
  - `extensions/ai-billing/README_EN.md`
  - `extensions/ai-billing/plugin.yaml`
- No runtime Go code or plugin behavior changes are intended.
