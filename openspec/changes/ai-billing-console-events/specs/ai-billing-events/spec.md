## MODIFIED Requirements

### Requirement: Billing plugin configuration

`ai-billing` SHALL support configuration for `billing_service`, `quota_scope`, `provider`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`. `billing_service` SHALL include `service_name`, `service_port`, `path`, `timeout`, and `auth_token`.

#### Scenario: Billing service target is configured

- **WHEN** `ai-billing` is configured with `billing_service.service_name`, `billing_service.service_port`, `billing_service.path`, `billing_service.timeout`, and `billing_service.auth_token`
- **THEN** the plugin SHALL create HTTP callouts to the configured billing-service endpoint using the configured timeout
- **AND** each billing-service callout SHALL include `content-type: application/json`
- **AND** each billing-service callout SHALL include `Authorization: Bearer <billing_service.auth_token>`

#### Scenario: Default fail policy is open

- **WHEN** `fail_policy` is omitted
- **THEN** `ai-billing` SHALL use `open` as the fail policy

#### Scenario: Default consumer header is x-mse-consumer

- **WHEN** `consumer_header` is omitted
- **THEN** `ai-billing` SHALL use `x-mse-consumer` as the consumer header name

### Requirement: Billing events are generated per completed AI request

`ai-billing` SHALL generate a request-level billing event for enabled AI requests after response completion. The plugin SHALL generate an `event_id` at request start, store it in request context, and reuse it when building the response-completion event. The `event_id` SHALL be a time-ordered random identifier in UUIDv7 or ULID format. The event `idempotency_key` SHALL default to the same value as `event_id`.

#### Scenario: Successful event includes request facts

- **WHEN** an enabled AI request completes and `X-Mse-Consumer` or the configured consumer header is present
- **THEN** `ai-billing` SHALL build an event containing `event_id`, `idempotency_key`, `request_id`, `consumer`, `route`, `provider`, `model`, `request_path`, `status_code`, `usage`, `usage_missing`, `start_time_ms`, `end_time_ms`, `is_stream`, `cluster`, and optional `price_version`
- **AND** `consumer` SHALL equal the configured consumer header value
- **AND** `request_id` SHALL be read from `x-request-id` or the Higress `x_request_id` property and used only for trace correlation
- **AND** the event SHALL NOT include raw API-key values, `tenant_id`, `user_id`, `api_key_id`, or `consumer_id` UUID fields

#### Scenario: Each AI request gets a new event identity

- **WHEN** two enabled AI requests are processed
- **THEN** `ai-billing` SHALL generate a different `event_id` for each request
- **AND** each `idempotency_key` SHALL equal that request's `event_id` by default

#### Scenario: Retry reuses idempotency key

- **WHEN** `ai-billing` retries delivery for the same billing event
- **THEN** every retry attempt SHALL reuse the original `idempotency_key`

#### Scenario: Structured token usage is present

- **WHEN** an enabled AI request completes with parsed token usage
- **THEN** `ai-billing` SHALL set `usage.unit` to `token`
- **AND** `usage.input` SHALL be the parsed prompt or input token count
- **AND** `usage.output` SHALL be the parsed completion or output token count
- **AND** `usage.total` SHALL be the parsed total token count when present
- **AND** `usage.total` SHALL fall back to `usage.input + usage.output` when parsed total token count is missing
- **AND** `usage.details` SHALL be an object
- **AND** `usage_missing` SHALL be `false`

#### Scenario: Usage is missing

- **WHEN** an enabled AI request completes without usable token usage
- **THEN** `ai-billing` SHALL emit an event with `usage.unit` set to `token`
- **AND** `usage.input`, `usage.output`, and `usage.total` SHALL be `0`
- **AND** `usage.details` SHALL be an empty object
- **AND** `usage_missing` SHALL be `true`

### Requirement: Billing event delivery is fail-open by default

`ai-billing` SHALL NOT block the user response when event delivery fails and `fail_policy` is `open`. `ai-billing` SHALL record billing delivery as failed for 401, 403, 408, 429, all 5xx responses, and dispatch or network errors.

#### Scenario: Billing service timeout

- **WHEN** the billing-service callout times out and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete
- **AND** `ai-billing` SHALL record the delivery failure

#### Scenario: Billing service returns server error

- **WHEN** billing-service returns a 5xx response and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete
- **AND** `ai-billing` SHALL record the delivery failure

#### Scenario: Billing service rejects authorization

- **WHEN** billing-service returns 401 or 403 and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete
- **AND** `ai-billing` SHALL record the delivery failure
- **AND** `ai-billing` SHALL NOT record the event as accepted

#### Scenario: Billing service throttles or times out request

- **WHEN** billing-service returns 408 or 429 and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete
- **AND** `ai-billing` SHALL record the delivery failure

#### Scenario: Billing service dispatch fails

- **WHEN** the billing-service callout fails because of a dispatch or network error and `fail_policy` is `open`
- **THEN** `ai-billing` SHALL allow the user response to complete
- **AND** `ai-billing` SHALL record the delivery failure

## ADDED Requirements

### Requirement: Billing plugin does not expose secrets or raw credentials

`ai-billing` SHALL NOT log `billing_service.auth_token` or raw API-key values. Documentation examples SHALL use placeholders for `billing_service.auth_token`.

#### Scenario: Auth token is configured

- **WHEN** `ai-billing` logs configuration, request handling, delivery success, or delivery failure information
- **THEN** the log output SHALL NOT contain the configured `billing_service.auth_token`

#### Scenario: Request contains raw API key

- **WHEN** an enabled AI request contains a raw API-key header
- **THEN** `ai-billing` SHALL NOT include the raw API-key value in the billing event payload
- **AND** `ai-billing` SHALL NOT write the raw API-key value to plugin logs

#### Scenario: Documentation shows auth token configuration

- **WHEN** `ai-billing` documentation or examples show `billing_service.auth_token`
- **THEN** the example value SHALL be a placeholder and SHALL NOT be a real shared secret
