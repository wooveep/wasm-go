## ADDED Requirements

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

The resource catalog SHALL include a complete `ai-billing` resource package.

#### Scenario: ai-billing resource files exist

- **WHEN** the resource catalog is inspected
- **THEN** `resources/plugins/ai-billing/README.md`, `resources/plugins/ai-billing/README_EN.md`, and `resources/plugins/ai-billing/spec.yaml` SHALL exist

#### Scenario: ai-billing documentation explains event delivery

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL explain request-level billing event generation, billing-service HTTP callout delivery, fail-open behavior, event fields, and that settlement and reconciliation belong to billing-service

#### Scenario: ai-billing independence is documented

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL state that `ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently and do not depend on each other's private runtime state

### Requirement: ai-billing resource schema matches plugin configuration

The resource catalog `spec.yaml` for `ai-billing` SHALL expose the current billing event plugin configuration fields and examples.

#### Scenario: Billing service fields are present in schema

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its config schema SHALL include `billing_service.service_name`, `billing_service.service_port`, `billing_service.path`, and `billing_service.timeout`

#### Scenario: Billing event fields are present in schema

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its config schema SHALL include `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`

#### Scenario: Billing example matches current plugin model

- **WHEN** `resources/plugins/ai-billing/spec.yaml` examples are inspected
- **THEN** they SHALL show `billing_service.service_name: modelfusion-console.higress-system.svc.cluster.local`, route/provider metadata, tenant and consumer headers, enabled AI path suffixes, and `fail_policy: open`

### Requirement: Resource metadata stays aligned with extension examples

Resource catalog metadata for `ai-quota` and `ai-billing` SHALL stay aligned with the corresponding extension examples.

#### Scenario: Resource examples follow extension plugin examples

- **WHEN** resource `spec.yaml` examples are compared with `extensions/ai-quota/plugin.yaml` and `extensions/ai-billing/plugin.yaml`
- **THEN** they SHALL use equivalent config field names, default values, and representative route-level `quota_scope` and `provider` examples

#### Scenario: Resource docs follow extension README content

- **WHEN** resource README files are compared with extension README files for the same plugin
- **THEN** the resource README files SHALL describe the same runtime responsibilities, supported configuration fields, and operational ownership boundaries
