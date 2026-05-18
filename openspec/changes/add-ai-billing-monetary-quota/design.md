## Context

`ai-quota` is currently a token-quota plugin. It reads `x-mse-consumer`, detects completion versus admin paths, exposes quota query/refresh/delta APIs under `admin_path`, and deducts total tokens from `redis_key_prefix + consumer` after token usage is parsed. This mixes data-plane admission with management-plane mutation and makes the gateway plugin the practical owner of quota state.

The target model separates responsibilities. Console and billing-service own tenants, consumers, balances, price strategies, billing statements, idempotency, reconciliation, and Redis cache refresh. Gateway plugins only use hot request-path data: `ai-quota` reads Redis balances and effective prices, `ai-billing` emits request billing events, and `ai-statistics` remains observability-only.

## Goals / Non-Goals

**Goals:**

- Replace token quota behavior in `ai-quota` with integer monetary balance admission and post-response deduction.
- Remove gateway-hosted quota management APIs and admin identity behavior from `ai-quota`.
- Support route-level `quota_scope` and provider selection through Higress WasmPlugin `matchRules`.
- Add `ai-billing` as an independent plugin that reports billing events to billing-service and defaults to fail-open.
- Keep `pkg/tokenusage` as the shared parser while avoiding shared private runtime state between AI plugins.
- Update docs, plugin metadata, and tests to reflect the new account and billing responsibilities.

**Non-Goals:**

- Implement strict pre-freeze, reservation, or guaranteed no-overdraft semantics in v1.
- Query billing-service or any database synchronously from `ai-quota`.
- Implement Console UI or billing-service DB schema and business workflows in this repository.
- Make `ai-statistics` a billing source of truth or runtime dependency for quota/billing.

## Decisions

1. Refactor `ai-quota` in place instead of adding a compatibility mode.

   The requirements intentionally remove token quota and admin quota management. Keeping both modes would preserve ambiguous ownership and increase test and documentation complexity. Existing admin API users must migrate to Console and billing-service APIs.

2. Use Redis hot keys for request-path decisions, with billing-service as the source of truth.

   `ai-quota` will build balance and price keys from templates such as `billing:balance:{tenant}:{quota_scope}:{consumer}` and `billing:effective_price:{tenant}:{provider}:{model}:{token_type}`. Redis loss or stale data is handled by billing-service rebuild and plugin missing-data policies, not gateway DB access.

3. Use integer monetary units and ceiling division for cost calculation.

   Balances and prices are integers, defaulting to `amount_scale: 1000000` and `price_unit_tokens: 1000000`. Cost is `ceil(tokens * price / price_unit_tokens)` for input and output tokens. This avoids floating-point drift in Wasm and Redis.

4. Keep pre-request admission intentionally lightweight in v1.

   Before forwarding, `ai-quota` only verifies that the Redis balance exists and is greater than zero unless `missing_balance_policy: allow` is configured. It does not estimate or reserve the request cost. This keeps latency low and matches the v1 non-goal of strict no-overdraft.

5. Deduct after usage is known, preferably through one Redis Lua operation.

   At response end, `ai-quota` parses usage via `pkg/tokenusage`, reads effective input/output prices, calculates the monetary cost, and deducts the balance. A Redis `EVAL` script should be used where practical to read prices and decrement balance atomically for one request. If usage or price is missing, configured policies decide whether to skip or deny future admission paths.

6. Make `ai-billing` independent from `ai-quota` and `ai-statistics`.

   `ai-billing` will parse usage directly with `pkg/tokenusage`, collect identity, route, cluster, request path, status code, request timing, stream flag, and optional gateway-calculated cost, then POST a billing event to billing-service. It must not depend on `ai-quota` context values or `ai-statistics` log/metric state.

7. Default `ai-billing` delivery to fail-open.

   Billing-service callout failures, timeouts, and 5xx responses are logged and counted but do not block the user response by default. Billing-service remains responsible for idempotency and reconciliation once events arrive.

## Risks / Trade-offs

- Loose pre-request admission can allow small overdrafts under concurrent traffic → Accept for v1, document clearly, and leave reservation keys for a later strict mode.
- Redis balance or price cache can be missing after restart → Default missing balance to deny, default missing price/usage to skip deduction, and require billing-service Redis rebuild from DB truth.
- Independent usage parsing in three plugins costs extra CPU and memory → Avoid private cross-plugin runtime state to preserve plugin independence; reuse `pkg/tokenusage` to keep field semantics aligned.
- Gateway-calculated cost can disagree with billing-service settlement → Treat gateway cost as advisory for reconciliation; billing-service recomputes from server-side price rules and DB facts.
- Removing admin APIs is a breaking operational change → Update docs and examples and call out migration to Console/billing-service management APIs.

## Migration Plan

1. Add billing-service support for account balances, tenant effective prices, billing event ingestion, idempotency, and Redis rebuild before enabling the new gateway behavior.
2. Deploy Redis key refresh for `billing:balance:*`, `billing:effective_price:*`, and `billing:price_version:{tenant}`.
3. Replace `ai-quota` configuration with monetary fields and route-level `matchRules`; remove `admin_consumer`, `admin_path`, and `redis_key_prefix`.
4. Deploy the new `ai-billing` plugin with fail-open callout to billing-service.
5. Remove clients and runbooks that call `/quota`, `/quota/refresh`, or `/quota/delta`; point management flows to Console and billing-service.
6. Roll back by reverting to the previous `ai-quota` image and token-quota config only if the old Redis token quota keys are still maintained during migration.

## Open Questions

- Which request header should be canonical for `request_id` when upstream providers do not return one?
- Should `ai-quota` include `billing:price_version:{tenant}` in gateway-calculated cost context for every request, or should only `ai-billing` report it?
- What metric names should be used for skipped deduction, missing usage, missing price, billing callout failure, and Redis deduction failure?
- What final priority should `ai-billing` use relative to `ai-statistics` and `ai-quota` in default plugin examples?
