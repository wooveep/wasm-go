## 1. Source Alignment

- [x] 1.1 Compare `resources/plugins/ai-quota` with `extensions/ai-quota/README.md`, `README_EN.md`, `plugin.yaml`, and current config parsing to list all stale fields and missing monetary fields.
- [x] 1.2 Compare `extensions/ai-billing/README.md`, `README_EN.md`, and `plugin.yaml` to identify the resource README and `spec.yaml` content that must be generated.
- [x] 1.3 Decide and document whether resource `readmeUrl` values should point to extension README files or resource README files.
- [x] 1.4 Decide and document whether resource `info.version` remains `1.0.0` or is bumped for the corrected resource metadata.

## 2. Update ai-quota Resource Package

- [x] 2.1 Rewrite `resources/plugins/ai-quota/README.md` to describe monetary balance admission, post-response deduction, Redis key conventions, configuration fields, and removal of gateway-hosted quota admin APIs.
- [x] 2.2 Rewrite `resources/plugins/ai-quota/README_EN.md` with equivalent English content.
- [x] 2.3 Replace obsolete `ai-quota` `spec.yaml` metadata descriptions with monetary quota descriptions.
- [x] 2.4 Replace obsolete `ai-quota` schema fields `redis_key_prefix`, `admin_consumer`, and `admin_path` with monetary fields: `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `balance_key_template`, `price_key_template`, `amount_scale`, `price_unit_tokens`, `enable_path_suffixes`, `missing_balance_policy`, `missing_price_policy`, and `missing_usage_policy`.
- [x] 2.5 Add `redis.database` to the `ai-quota` Redis schema and keep `redis.service_name` as the required Redis field.
- [x] 2.6 Update `ai-quota` examples to show monetary balance and effective price key configuration plus enabled AI path suffixes.
- [x] 2.7 Align `ai-quota` resource priority and example values with `extensions/ai-quota/plugin.yaml`, unless task 1.3 or 1.4 records a deliberate catalog-specific difference.

## 3. Generate ai-billing Resource Package

- [x] 3.1 Create `resources/plugins/ai-billing/README.md` based on `extensions/ai-billing/README.md`, including event delivery behavior, event fields, billing-service ownership, fail-open behavior, and independent deployment.
- [x] 3.2 Create `resources/plugins/ai-billing/README_EN.md` with equivalent English content based on `extensions/ai-billing/README_EN.md`.
- [x] 3.3 Create `resources/plugins/ai-billing/spec.yaml` with resource metadata for the `ai-billing` plugin.
- [x] 3.4 Add `billing_service` schema fields: `service_name`, `service_port`, `path`, and `timeout`, with `service_name` required.
- [x] 3.5 Add `ai-billing` event config fields: `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`.
- [x] 3.6 Add `ai-billing` examples that match `extensions/ai-billing/plugin.yaml`, including `billing_service.service_name: modelfusion-console.higress-system.svc.cluster.local`, route-level `quota_scope`, provider, enabled AI path suffixes, and `fail_policy: open`.

## 4. Verification

- [x] 4.1 Parse `resources/plugins/ai-quota/spec.yaml` and `resources/plugins/ai-billing/spec.yaml` as YAML.
- [x] 4.2 Search `resources/plugins/ai-quota` to confirm `redis_key_prefix`, `admin_consumer`, `admin_path`, `/quota/refresh`, and `/quota/delta` do not appear as supported configuration or supported APIs.
- [x] 4.3 Search `resources/plugins/ai-billing` to confirm required billing-service, identity, path suffix, and fail policy fields are documented in README files and `spec.yaml`.
- [x] 4.4 Run `openspec validate update-ai-plugin-resource-docs --strict`.
- [x] 4.5 Run `graphify update .` after resource documentation changes.
- [x] 4.6 Review `git diff -- resources/plugins openspec/changes/update-ai-plugin-resource-docs` to confirm the change is limited to the intended documentation and resource metadata scope.
