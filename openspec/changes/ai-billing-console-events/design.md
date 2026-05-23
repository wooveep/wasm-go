## Context

`ai-billing` already emits request-level billing events from response completion paths in `extensions/ai-billing/main.go`, using `pkg/tokenusage` to parse token counts and an internal HTTP client to deliver events to the configured billing service. The Console ingestion path now needs an authenticated, idempotent, consumer-oriented payload that can safely support retries and missing-usage handling without exposing secrets or raw API keys.

The change is constrained by WASM plugin execution: the event identity must be created while request context is available, the response-time payload must reuse stored request state, and billing delivery must remain fail-open so user AI traffic is not blocked by Console availability.

## Goals / Non-Goals

**Goals:**

- Add `billing_service.auth_token` configuration and use it only to set `Authorization: Bearer <token>` on billing callouts.
- Generate one time-ordered random `event_id` per AI request at request start and reuse it as the default `idempotency_key`.
- Attribute events using the configured consumer header, defaulting to `x-mse-consumer`, and report the header value as `consumer`.
- Emit the Console billing event payload with structured token usage and deterministic missing-usage behavior.
- Classify 401, 403, 408, 429, all 5xx responses, and dispatch/network errors as delivery failures while preserving fail-open semantics.
- Prevent plugin logs and docs examples from exposing `auth_token` or raw API-key values.

**Non-Goals:**

- Implement backend Console ingestion, pricing, compensation, or settlement behavior.
- Report raw API keys, tenant IDs, user IDs, API-key IDs, or internal consumer UUIDs.
- Add balance mutation, Redis deductions, or coupling to `ai-quota` / `ai-statistics`.
- Define future usage detail fields beyond reserving `usage.details` for later extensions.

## Decisions

1. Generate event identity at request start.

   The plugin will create `event_id` in the request-header phase for enabled paths and store it in the request context together with start time and request facts needed later. This makes streaming and non-streaming response paths share the same event identity. The default `idempotency_key` will be copied from `event_id`; any future internal retry mechanism must reuse the stored key.

   Alternative considered: generate the ID at response completion. That is simpler but makes retry reuse and request-start correlation weaker, especially for streaming responses and partial failures.

2. Use a time-ordered random ID format.

   The implementation should use UUIDv7 or ULID. UUIDv7 is preferred when an existing lightweight dependency or local helper is available because it is standardized and time-sortable. If adding a dependency is undesirable for the WASM plugin, a small internal ULID/UUIDv7 generator backed by request time and secure randomness is acceptable. Tests should validate uniqueness and format/time-order properties without relying on a fixed generated value.

   Alternative considered: reuse `x-request-id` as the event identity. That does not guarantee uniqueness, ordering, or idempotency ownership by the billing plugin, so it remains only a trace correlation field.

3. Treat `X-Mse-Consumer` as the billing identity entrance.

   The payload will use the configured `consumer_header` value directly as `consumer`, with the default header name `x-mse-consumer`. The plugin will not parse or forward original API-key headers and will not report tenant/user/API-key/consumer UUID fields in the Console event payload. `tenant_header` may remain in config for compatibility, but this change does not require sending `tenant_id`.

   Alternative considered: derive consumer from API-key or quota state. That would couple billing to private auth/quota internals and increases credential exposure risk.

4. Serialize structured usage.

   The event payload will contain `usage.unit = "token"`, numeric `input`, `output`, `total`, and an object `details`. Parsed prompt/input tokens map to `input`; completion/output tokens map to `output`; parsed total maps to `total`, with `input + output` as fallback when total is missing. When no usable token usage is parsed, all numeric values are `0`, `details` is `{}`, and `usage_missing` is `true`.

   Alternative considered: keep flat token count fields. That is less extensible for cached, reasoning, audio, and provider-specific usage details.

5. Keep delivery fail-open but make failure classification explicit.

   Billing callouts will include JSON content type and Bearer authorization headers. Delivery will be considered failed for 401, 403, 408, 429, every 5xx status, and dispatch/network errors. 401 and 403 must not be logged or counted as accepted. Fail-open behavior means those failures are recorded internally but the user AI response continues.

   Alternative considered: fail closed for authentication failures. That would protect billing completeness but risks breaking user traffic for configuration or Console availability issues, contrary to the existing default fail-open contract.

## Risks / Trade-offs

- Secret leakage through logs or examples -> keep `auth_token` out of formatted config dumps, redact any delivery log fields that could contain authorization headers, and use placeholders in docs/tests.
- ID generation dependency bloat -> prefer an existing dependency or a small local generator; keep the public contract at UUIDv7/ULID rather than binding specs to one package.
- Missing token usage can still create ingestion load -> emit explicit zero usage with `usage_missing=true` so Console can record, compensate, or skip deterministically.
- Header case differences can break consumer attribution -> normalize configured header lookup through existing Higress header APIs and test both `X-Mse-Consumer` and `x-mse-consumer` behavior where the test host supports it.
- Future retries can accidentally create duplicate charges -> store `idempotency_key` with event state and design retry helpers to receive the existing event rather than rebuilding identity.

## Migration Plan

1. Add `auth_token` to `billing_service` configuration with documentation showing only `<shared-secret>` or similar placeholders.
2. Deploy Console support for Bearer-authenticated structured events before enabling the updated plugin configuration.
3. Roll out plugin config containing `billing_service.auth_token` from the deployment Secret.
4. Validate Console accepts events with `consumer`, structured `usage`, and idempotency fields.
5. Rollback by reverting the plugin/config to the previous event contract if Console ingestion rejects the new schema.

## Open Questions

- Whether the final implementation should standardize on UUIDv7 or ULID depends on current dependency constraints and existing helper availability at implementation time.
- Whether `auth_token` should be mandatory at parse time or only required for successful delivery depends on how staged Console/plugin rollout will be handled.
