## Why

`ai-billing` needs a stable Console-facing event contract so billing ingestion can authenticate plugin callouts, de-duplicate events, attribute usage by Higress consumer, and make retry behavior safe. The current billing event requirements do not fully specify the auth token, idempotent event identity, structured usage object, or delivery-failure semantics needed by the Console API.

## What Changes

- Add `billing_service.auth_token` as a deployment-provided shared secret used only for Console callout authorization.
- Require billing callouts to include `content-type: application/json` and `Authorization: Bearer <auth_token>`.
- Generate a time-ordered random `event_id` at request start, store it in request context, reuse it after response completion, and default `idempotency_key` to the same value.
- Use `X-Mse-Consumer` / configured `consumer_header` as the sole reported consumer value, without reading or reporting raw API keys or internal user identifiers.
- Replace flat token usage reporting with a structured `usage` object and explicit `usage_missing` behavior.
- Define delivery failures for 401, 403, 408, 429, 5xx, and dispatch/network errors while keeping fail-open behavior.
- Require logging and documentation examples to avoid exposing `auth_token` or raw API-key values.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ai-billing-events`: tighten the billing plugin configuration, event identity, payload schema, consumer attribution, auth header, usage reporting, and delivery outcome requirements.

## Impact

- Affected plugin code: `ai-billing` configuration parsing, request context state, response completion handling, event serialization, token usage mapping, HTTP callout headers, and delivery result logging.
- Affected docs/examples: `ai-billing` configuration examples must show only placeholder `auth_token` values and must not include real secrets.
- Affected tests: configuration parsing, event ID/idempotency generation, consumer header capture, structured usage payloads, missing usage payloads, authorization header inclusion, and delivery-failure classification.
- External systems: Console billing ingestion must accept the structured event payload and Bearer token authentication contract.
