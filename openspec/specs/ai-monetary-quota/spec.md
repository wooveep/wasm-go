# ai-monetary-quota Specification

## Purpose
TBD - created by archiving change add-ai-billing-monetary-quota. Update Purpose after archive.
## Requirements
### Requirement: Monetary quota configuration

`ai-quota` SHALL support monetary quota configuration using Redis connection settings, `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `balance_key_template`, `price_key_template`, `amount_scale`, `price_unit_tokens`, `enable_path_suffixes`, `missing_balance_policy`, `missing_price_policy`, and `missing_usage_policy`.

#### Scenario: Default monetary configuration is applied

- **WHEN** `ai-quota` is configured with Redis settings and omits optional monetary fields
- **THEN** the plugin SHALL default `quota_scope` to `global`, `tenant_header` to `x-mse-tenant`, `consumer_header` to `x-mse-consumer`, `balance_key_template` to `billing:balance:{tenant}:{quota_scope}:{consumer}`, `price_key_template` to `billing:effective_price:{tenant}:{provider}:{model}:{token_type}`, `amount_scale` to `1000000`, `price_unit_tokens` to `1000000`, `missing_balance_policy` to `deny`, `missing_price_policy` to `skip`, and `missing_usage_policy` to `skip`

#### Scenario: Route-level quota scope is selected

- **WHEN** a Higress WasmPlugin `matchRules` entry matches an AI route and provides `quota_scope` and `provider`
- **THEN** `ai-quota` SHALL use the matched `quota_scope` and `provider` for balance and price key construction for that request

### Requirement: Legacy quota management is removed

`ai-quota` SHALL NOT expose gateway-hosted quota management endpoints or require admin quota configuration.

#### Scenario: Admin quota paths are not handled

- **WHEN** a request path ends with `/quota`, `/quota/refresh`, or `/quota/delta`
- **THEN** `ai-quota` SHALL NOT execute quota query, refresh, delta, admin identity, or request-body quota mutation logic

#### Scenario: Admin configuration is absent

- **WHEN** `ai-quota` configuration omits `admin_consumer` and `admin_path`
- **THEN** the plugin SHALL parse configuration successfully if the required Redis settings are valid

### Requirement: Request admission uses Redis monetary balance

`ai-quota` SHALL perform request admission for enabled AI path suffixes by reading a Redis monetary balance key built from tenant, quota scope, and consumer identity.

#### Scenario: Positive balance allows request

- **WHEN** an enabled AI request has tenant and consumer headers and Redis returns a balance greater than `0`
- **THEN** `ai-quota` SHALL allow the request to continue

#### Scenario: Non-positive balance denies request

- **WHEN** an enabled AI request has tenant and consumer headers and Redis returns a balance less than or equal to `0`
- **THEN** `ai-quota` SHALL deny the request

#### Scenario: Missing balance follows policy

- **WHEN** an enabled AI request has tenant and consumer headers and the Redis balance key is missing
- **THEN** `ai-quota` SHALL deny the request when `missing_balance_policy` is `deny` and SHALL allow the request when `missing_balance_policy` is `allow`

#### Scenario: Missing identity denies request

- **WHEN** an enabled AI request does not include the configured tenant or consumer header
- **THEN** `ai-quota` SHALL deny the request

### Requirement: Monetary deduction uses token usage and effective prices

`ai-quota` SHALL deduct monetary balance after response completion when token usage and effective prices are available.

#### Scenario: Usage and prices produce cost

- **WHEN** response completion provides input tokens, output tokens, and model, and Redis contains matching input and output effective prices
- **THEN** `ai-quota` SHALL calculate `ceil(input_tokens * input_price / price_unit_tokens) + ceil(output_tokens * output_price / price_unit_tokens)` and deduct that integer amount from the Redis balance key

#### Scenario: Usage is missing

- **WHEN** response completion does not provide usable token usage
- **THEN** `ai-quota` SHALL skip deduction when `missing_usage_policy` is `skip`

#### Scenario: Price is missing

- **WHEN** response completion provides usage but Redis does not contain a required effective price key
- **THEN** `ai-quota` SHALL skip deduction when `missing_price_policy` is `skip`

### Requirement: Billing-service remains the account source of truth

`ai-quota` SHALL treat Redis as request-path hot data and SHALL NOT directly manage account balances, tenant price rules, billing statements, or database state.

#### Scenario: Redis cache is rebuilt externally

- **WHEN** Redis hot balance or effective price keys are lost
- **THEN** billing-service SHALL be responsible for rebuilding Redis from database-backed balances and price rules, and `ai-quota` SHALL apply its missing-data policies until keys are restored

### Requirement: Token usage parser is shared but runtime state is not

`ai-quota` SHALL use `pkg/tokenusage` for token usage extraction and SHALL NOT depend on private runtime state from `ai-statistics` or `ai-billing`.

#### Scenario: Statistics plugin is disabled

- **WHEN** `ai-statistics` is not enabled for a route
- **THEN** `ai-quota` SHALL still perform admission and eligible post-response deduction using its own invocation of `pkg/tokenusage`

