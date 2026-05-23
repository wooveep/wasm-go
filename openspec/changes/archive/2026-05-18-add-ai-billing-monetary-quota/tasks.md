## 1. Impact And Test Planning

- [x] 1.1 Run GitNexus impact analysis for `ai-quota` symbols that will be edited, including config parsing, request header processing, response body processing, and legacy admin quota handlers.
- [x] 1.2 Run GitNexus impact analysis for shared wrapper or token usage symbols before editing them, if implementation requires shared-library changes.
- [x] 1.3 Identify existing `ai-quota` and `ai-statistics` tests that cover request admission, response usage parsing, Redis calls, and HTTP callouts.
- [x] 1.4 Add failing tests or test cases for monetary quota config defaults, missing identity, positive balance, non-positive balance, missing balance policy, cost calculation, missing usage, and missing price.
- [x] 1.5 Add failing tests or test cases for `ai-billing` config defaults, event payload construction, usage-missing payloads, callout success, callout timeout, and fail-open 5xx behavior.

## 2. Refactor ai-quota Configuration And Admission

- [x] 2.1 Replace token quota config fields with monetary config fields and validation in `extensions/ai-quota`.
- [x] 2.2 Remove `admin_consumer`, `admin_path`, `redis_key_prefix`, `ChatModeAdmin`, `AdminMode*`, and admin request-body quota mutation paths from `ai-quota`.
- [x] 2.3 Implement enabled AI path suffix detection without admin path handling.
- [x] 2.4 Build balance Redis keys from `balance_key_template`, tenant header, quota scope, and consumer header.
- [x] 2.5 Implement request admission using Redis balance and `missing_balance_policy`.
- [x] 2.6 Update denial responses and logs for missing identity, missing balance, Redis errors, and non-positive balance.

## 3. Implement ai-quota Monetary Deduction

- [x] 3.1 Parse response token usage and model in `ai-quota` using `pkg/tokenusage` without relying on `ai-statistics`.
- [x] 3.2 Build input and output price keys from `price_key_template`, tenant, provider, model, and token type.
- [x] 3.3 Implement integer ceiling cost calculation using `price_unit_tokens`.
- [x] 3.4 Deduct the computed monetary cost from Redis balance, preferably through one `EVAL` operation that reads prices and updates balance atomically.
- [x] 3.5 Implement `missing_usage_policy` and `missing_price_policy` behavior with logs or metrics for skipped deductions.
- [x] 3.6 Update `ai-quota` plugin metadata, sample YAML, README files, and docs to describe monetary quota and removed admin APIs.

## 4. Add ai-billing Plugin

- [x] 4.1 Create `extensions/ai-billing` module structure following existing extension conventions.
- [x] 4.2 Implement `ai-billing` config parsing for billing-service target, quota scope, provider, identity headers, enabled path suffixes, and fail policy.
- [x] 4.3 Capture request start time, request path, stream flag, route, cluster, status code, and identity data needed for billing events.
- [x] 4.4 Parse response token usage and model with `pkg/tokenusage` inside `ai-billing`.
- [x] 4.5 Construct billing event JSON with request ID, idempotency key, tenant, consumer, quota scope, provider, model, route, cluster, token counts, timing, usage-missing flag, optional price version, and optional gateway-calculated cost.
- [x] 4.6 Deliver billing events to billing-service with HTTP callout and configured timeout.
- [x] 4.7 Implement fail-open behavior for timeout, network failure, and 5xx responses, including logs or metrics.
- [x] 4.8 Add `ai-billing` plugin metadata, sample YAML, README files, and docs.

## 5. Integration And Verification

- [x] 5.1 Ensure `ai-quota`, `ai-billing`, and `ai-statistics` each work when the other two plugins are disabled.
- [x] 5.2 Run unit tests for `extensions/ai-quota`, `extensions/ai-billing`, and any changed shared packages.
- [x] 5.3 Run formatting and static checks used by the repository for changed Go modules.
- [x] 5.4 Run OpenSpec validation for `add-ai-billing-monetary-quota`.
- [x] 5.5 Run `graphify update .` after code changes to refresh the project graph.
- [x] 5.6 Run `gitnexus_detect_changes()` or the GitNexus detect changes tool before committing to verify affected symbols and execution flows are expected.
