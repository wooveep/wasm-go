## 1. Pre-Change Analysis

- [x] 1.1 Run GitNexus impact analysis for each `ai-billing` symbol that will be edited, including `parseConfig`, request-header handling, response-completion handling, event delivery, and `recordUsage`.
- [x] 1.2 Review `extensions/ai-billing/main.go`, `extensions/ai-billing/main_test.go`, `pkg/tokenusage`, and existing `ai-billing` docs to confirm exact edit points.
- [x] 1.3 Decide whether to implement UUIDv7 or ULID based on existing dependencies and WASM plugin dependency constraints.

## 2. Configuration And Event State

- [x] 2.1 Add `billing_service.auth_token` to the `ai-billing` configuration model and parser without logging the configured value.
- [x] 2.2 Preserve defaults for `consumer_header`, `tenant_header`, `quota_scope`, `provider`, `enable_path_suffixes`, and `fail_policy`.
- [x] 2.3 Generate one UUIDv7 or ULID `event_id` at request start for enabled AI request paths.
- [x] 2.4 Store `event_id`, default `idempotency_key`, start time, request ID, consumer header value, request path, route, provider, cluster, and stream state in request context for response completion.

## 3. Payload Construction

- [x] 3.1 Replace the current billing event payload fields with the Console event schema: `event_id`, `idempotency_key`, `request_id`, `consumer`, `route`, `provider`, `model`, `request_path`, `status_code`, `usage`, `usage_missing`, `start_time_ms`, `end_time_ms`, `is_stream`, `cluster`, and optional `price_version`.
- [x] 3.2 Populate `consumer` from the configured consumer header, defaulting to `x-mse-consumer`, and verify `X-Mse-Consumer: consumer-a` produces `"consumer":"consumer-a"`.
- [x] 3.3 Keep `request_id` sourced from `x-request-id` or Higress `x_request_id` property only for correlation.
- [x] 3.4 Ensure the payload does not include raw API-key values, `tenant_id`, `user_id`, `api_key_id`, or `consumer_id` UUID fields.
- [x] 3.5 Map parsed token counts to `usage.unit`, `usage.input`, `usage.output`, `usage.total`, and `usage.details`, using `input + output` when total is missing.
- [x] 3.6 Emit `usage_missing=true` with zero token counts and empty `usage.details` when no usable token usage is parsed.

## 4. Delivery And Failure Handling

- [x] 4.1 Add `content-type: application/json` and `Authorization: Bearer <billing_service.auth_token>` to billing-service callouts.
- [x] 4.2 Classify 401, 403, 408, 429, all 5xx responses, and dispatch/network errors as delivery failures.
- [x] 4.3 Ensure 401 and 403 responses are not recorded as accepted events.
- [x] 4.4 Preserve fail-open behavior for all delivery failures when `fail_policy` is `open`.
- [x] 4.5 Structure any future retry helper so the same event reuses the original `idempotency_key`.

## 5. Tests And Documentation

- [x] 5.1 Update config parsing tests for `billing_service.auth_token` and default `consumer_header`.
- [x] 5.2 Add event identity tests proving each AI request gets a new UUIDv7/ULID and `idempotency_key` defaults to `event_id`.
- [x] 5.3 Update delivery tests to assert JSON content type and Bearer authorization headers.
- [ ] 5.4 Add payload tests for `X-Mse-Consumer`, structured usage, total fallback, missing usage, and excluded credential/identity fields.
- [ ] 5.5 Add delivery result tests for 401, 403, 408, 429, 5xx, and dispatch/network errors.
- [ ] 5.6 Update `ai-billing` documentation and examples to include `auth_token: <shared-secret>` placeholders only.
- [ ] 5.7 Verify plugin logs and test assertions do not expose `auth_token` or raw API-key values.

## 6. Verification

- [ ] 6.1 Run targeted `ai-billing` tests.
- [ ] 6.2 Run broader affected package tests for `pkg/tokenusage` and shared HTTP/test helpers if touched.
- [ ] 6.3 Run `graphify update .` after code changes.
- [ ] 6.4 Run `gitnexus_detect_changes()` before committing to confirm affected symbols and execution flows match the expected `ai-billing` scope.
- [ ] 6.5 Run `openspec status --change ai-billing-console-events` and confirm all required artifacts are complete.
