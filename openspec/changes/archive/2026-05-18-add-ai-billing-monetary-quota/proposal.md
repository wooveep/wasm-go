## Why

`ai-quota` currently models quota as token capacity and exposes quota management APIs from inside the gateway plugin, which makes it difficult to integrate with Console-owned accounts, balances, tenant-specific prices, and auditable billing statements. The gateway should move request-path quota checks to Redis hot data while billing-service remains the source of truth for balances, prices, billing events, idempotency, and reconciliation.

## What Changes

- **BREAKING**: Refactor `ai-quota` from token quota management to monetary balance admission and post-response monetary deduction.
- **BREAKING**: Remove `ai-quota` HTTP quota management endpoints, including `/quota`, `/quota/refresh`, and `/quota/delta`.
- **BREAKING**: Remove `ai-quota` `admin_consumer`, `admin_path`, admin identity checks, and request-body quota mutation behavior.
- Add `ai-quota` configuration for `quota_scope`, provider, tenant/consumer headers, Redis key templates, amount scale, price unit, enabled AI path suffixes, and missing-data policies.
- Make `ai-quota` use Higress WasmPlugin `matchRules` so different AI routes can bind different `quota_scope` and provider values.
- Make `ai-quota` read Redis hot balances before forwarding requests, default-deny missing or non-positive balances, then compute and deduct monetary cost after usage is available.
- Add a new independent `ai-billing` plugin that parses token usage, constructs request-level billing events, and reports them to billing-service through HTTP callout.
- Keep `ai-statistics`, `ai-quota`, and `ai-billing` independently deployable and independently responsible for token usage parsing through `pkg/tokenusage`.
- Update plugin documentation and examples to describe monetary quota, billing events, Redis key conventions, billing-service responsibilities, and the removal of gateway-hosted quota management APIs.

## Capabilities

### New Capabilities

- `ai-monetary-quota`: Monetary balance admission and post-response deduction for AI requests using Redis hot balance and tenant-specific effective prices.
- `ai-billing-events`: Independent billing event generation and fail-open delivery from the gateway to billing-service.

### Modified Capabilities

None.

## Impact

- Affected plugins: `extensions/ai-quota`, new `extensions/ai-billing`, and documentation under `docs/plugins/ai`.
- Shared libraries: `pkg/tokenusage` remains the shared usage parsing contract; Redis and HTTP wrapper APIs may be used by both quota and billing plugins.
- Runtime behavior: gateway quota decisions now depend on Redis monetary balance keys and optional Redis effective price keys; billing-service owns DB truth, Redis refresh, tenant price strategy, idempotency, and reconciliation.
- Operational behavior: existing token-quota admin workflows and clients that call `ai-quota` quota management endpoints must migrate to Console and billing-service APIs.
