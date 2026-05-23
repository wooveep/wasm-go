# ai-billing-events Specification

## Purpose
TBD - created by archiving change add-ai-billing-monetary-quota. Update Purpose after archive.
## Requirements
### Requirement: Billing plugin configuration

`ai-billing` SHALL support configuration for `billing_service`, `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`.

#### Scenario: Billing service target is configured

- **WHEN** `ai-billing` is configured with `billing_service.service_name`, `billing_service.service_port`, `billing_service.path`, and `billing_service.timeout`
- **THEN** the plugin SHALL create HTTP callouts to the configured billing-service endpoint using the configured timeout

#### Scenario: Default fail policy is open

- **WHEN** `fail_policy` is omitted
- **THEN** `ai-billing` SHALL use `open` as the fail policy

### Requirement: Billing events are generated per completed AI request

`ai-billing` SHALL generate a request-level billing event for enabled AI requests after response completion.

#### Scenario: Successful event includes request facts

- **WHEN** an enabled AI request completes and identity headers are present
- **THEN** `ai-billing` SHALL build an event containing request ID, idempotency key, tenant, consumer, quota scope, provider, model, route, cluster, request path, status code, start time, end time, stream flag, token usage fields, usage-missing flag, optional price version, and optional gateway-calculated cost

#### Scenario: Usage is missing

- **WHEN** an enabled AI request completes without usable token usage
- **THEN** `ai-billing` SHALL emit an event with `usage_missing` set to `true` and token counts set to `0` or omitted

### Requirement: Billing event delivery is fail-open by default

`ai-billing` SHALL NOT block the user response when event delivery fails and `fail_policy` is `open`.

#### Scenario: Billing service timeout

- **WHEN** the billing-service callout times out and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete and SHALL record the delivery failure

#### Scenario: Billing service returns server error

- **WHEN** billing-service returns a 5xx response and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete and SHALL record the delivery failure

### Requirement: Billing plugin does not mutate account balance

`ai-billing` SHALL NOT deduct Redis balances or directly mutate account balances.

#### Scenario: Event is accepted

- **WHEN** billing-service accepts a billing event
- **THEN** `ai-billing` SHALL NOT perform any Redis balance deduction or database account update

### Requirement: Billing plugin is independent from quota and statistics plugins

`ai-billing` SHALL parse token usage with `pkg/tokenusage` and SHALL NOT depend on private runtime state from `ai-quota` or `ai-statistics`.

#### Scenario: Quota plugin is disabled

- **WHEN** `ai-quota` is not enabled for a route
- **THEN** `ai-billing` SHALL still generate and deliver billing events for enabled AI requests

#### Scenario: Statistics plugin is disabled

- **WHEN** `ai-statistics` is not enabled for a route
- **THEN** `ai-billing` SHALL still generate and deliver billing events for enabled AI requests

