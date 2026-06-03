# ai-plugin-resource-docs Specification

## Purpose
Define resource catalog documentation and schema expectations for AI-related plugins so operator-facing examples stay aligned with extension runtime behavior.
## Requirements
### Requirement: ai-quota resource documentation reflects monetary quota behavior

The resource catalog documentation for `ai-quota` SHALL describe the current monetary balance admission and post-response deduction behavior.

#### Scenario: Monetary quota overview is documented

- **WHEN** an operator reads `resources/plugins/ai-quota/README.md` or `resources/plugins/ai-quota/README_EN.md`
- **THEN** the documentation SHALL explain that `ai-quota` reads Redis hot balances before forwarding enabled AI requests and deducts monetary cost after response token usage is available

#### Scenario: Removed admin APIs are not presented as supported

- **WHEN** an operator reads the `ai-quota` resource documentation
- **THEN** the documentation SHALL NOT present `/quota`, `/quota/refresh`, `/quota/delta`, `admin_consumer`, `admin_path`, or `redis_key_prefix` as supported configuration or management interfaces

#### Scenario: Billing ownership is documented

- **WHEN** an operator reads the `ai-quota` resource documentation
- **THEN** the documentation SHALL state that account balances, prices, billing statements, Redis rebuilds, idempotency, and reconciliation are owned by Console or billing-service rather than the gateway plugin

### Requirement: ai-quota resource schema matches monetary quota configuration

The resource catalog `spec.yaml` for `ai-quota` SHALL expose the current monetary quota configuration fields and examples.

#### Scenario: Monetary fields are present in schema

- **WHEN** `resources/plugins/ai-quota/spec.yaml` is inspected
- **THEN** its config schema SHALL include `redis`, `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `balance_key_template`, `price_key_template`, `amount_scale`, `price_unit_tokens`, `enable_path_suffixes`, `missing_balance_policy`, `missing_price_policy`, and `missing_usage_policy`

#### Scenario: Redis schema includes database

- **WHEN** `resources/plugins/ai-quota/spec.yaml` is inspected
- **THEN** its Redis object schema SHALL include `service_name`, `service_port`, `username`, `password`, `timeout`, and `database`, with `service_name` required

#### Scenario: Obsolete fields are removed from schema

- **WHEN** `resources/plugins/ai-quota/spec.yaml` is inspected
- **THEN** its schemas and examples SHALL NOT include `redis_key_prefix`, `admin_consumer`, or `admin_path`

### Requirement: ai-billing resource package is generated

The resource catalog SHALL include a complete `ai-billing` resource package that reflects the current `extensions/ai-billing` implementation.

#### Scenario: ai-billing resource files exist

- **WHEN** the resource catalog is inspected
- **THEN** `resources/plugins/ai-billing/README.md`, `resources/plugins/ai-billing/README_EN.md`, and `resources/plugins/ai-billing/spec.yaml` SHALL exist

#### Scenario: ai-billing documentation explains event delivery

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL explain request-level billing event generation, billing-service HTTP callout delivery, Bearer token authorization from `billing_service.auth_token`, fail-open behavior, current implemented event fields, the global-default plus rule-override configuration model, and that settlement and reconciliation belong to billing-service

#### Scenario: Config inheritance is documented

- **WHEN** an operator reads `resources/plugins/ai-billing/README.md` or `resources/plugins/ai-billing/README_EN.md`
- **THEN** the documentation SHALL explain that `defaultConfig` contains shared billing-service and identity defaults, while `matchRules[].config` can contain only route-specific overrides such as `provider`, `quota_scope`, `enable_path_suffixes`, or `fail_policy`

#### Scenario: ai-billing event documentation matches implemented payload

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL document the emitted event fields as `event_id`, `idempotency_key`, `request_id`, `tenant`, `consumer`, `quota_scope`, `route`, `provider`, `model`, `request_path`, `status_code`, `usage`, `usage_missing`, `start_time_ms`, `end_time_ms`, `is_stream`, `cluster`, and optional `price_version`

#### Scenario: ai-billing removed event fields are not documented as emitted

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL NOT state that billing events emit top-level `input_tokens`, top-level `output_tokens`, top-level `total_tokens`, or `gateway_calculated_cost`

#### Scenario: ai-billing independence is documented

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL state that `ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently and do not depend on each other's private runtime state

### Requirement: ai-billing resource schema matches plugin configuration

The resource catalog `spec.yaml` for `ai-billing` SHALL expose the current billing event plugin configuration fields, inherited rule override behavior, and examples.

#### Scenario: Billing service fields are present in schema

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its config schema SHALL include `billing_service.service_name`, `billing_service.service_port`, `billing_service.path`, `billing_service.timeout`, and `billing_service.auth_token`

#### Scenario: Billing event configuration fields are present in schema

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its config schema SHALL include `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`

#### Scenario: Route config schema allows partial overrides

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its route config schema SHALL allow `provider`, `quota_scope`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy` without requiring `billing_service`

#### Scenario: Billing service auth token examples are placeholders

- **WHEN** `resources/plugins/ai-billing/spec.yaml` examples are inspected
- **THEN** every `billing_service.auth_token` example value SHALL use a placeholder such as `<shared-secret>` rather than a real secret

#### Scenario: Billing example matches inherited config model

- **WHEN** `resources/plugins/ai-billing/spec.yaml` examples are inspected
- **THEN** they SHALL show global `billing_service.service_name: modelfusion-console.higress-system.svc.cluster.local`, `billing_service.service_port: 8080`, `billing_service.path: /internal/billing/events`, `billing_service.timeout: 750`, shared tenant and consumer headers, shared enabled AI path suffixes, `fail_policy: open`, and route-level `quota_scope` and `provider` overrides

### Requirement: Resource metadata stays aligned with extension examples

Resource catalog metadata for `ai-quota` and `ai-billing` SHALL stay aligned with the corresponding extension examples.

#### Scenario: Resource examples follow extension plugin examples

- **WHEN** resource `spec.yaml` examples are compared with `extensions/ai-quota/plugin.yaml` and `extensions/ai-billing/plugin.yaml`
- **THEN** they SHALL use equivalent config field names, default values, secret placeholders, and representative route-level `quota_scope` and `provider` examples

#### Scenario: ai-billing extension example uses recommended override shape

- **WHEN** `extensions/ai-billing/plugin.yaml` is inspected
- **THEN** it SHALL put common `billing_service`, identity headers, enabled AI path suffixes, and `fail_policy` in `defaultConfig`, while `matchRules[].config` contains only route-specific differences such as `quota_scope` and `provider`

#### Scenario: Resource docs follow extension README content

- **WHEN** resource README files are compared with extension README files for the same plugin
- **THEN** the resource README files SHALL describe the same runtime responsibilities, supported configuration fields, inherited rule override behavior, and operational ownership boundaries

#### Scenario: ai-billing resource docs follow implementation tests

- **WHEN** `resources/plugins/ai-billing` event field documentation is compared with `extensions/ai-billing/main.go` and `extensions/ai-billing/main_test.go`
- **THEN** the resource documentation SHALL follow the implemented serialized payload and SHALL NOT reintroduce fields that tests assert are excluded
