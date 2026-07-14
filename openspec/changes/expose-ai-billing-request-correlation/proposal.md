## Why

Console relays Model Experience traffic through Higress, where Envoy creates the canonical request ID recorded by `ai-billing`. The relay cannot currently observe that trusted ID, so its UI identifier cannot be correlated with the billing event.

## What Changes

- Replace a trusted internal response header with the canonical request ID already held by `ai-billing`.
- Ensure provider or caller values under that header cannot spoof the correlation identity.
- Verify the header for non-streaming, streaming, and gateway-local response paths without changing billing event semantics.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ai-billing-events`: Billing responses expose the event correlation request ID to trusted internal relays.

## Impact

Affected code is limited to `extensions/ai-billing` response-header handling, its tests, and documentation. Billing payloads, event IDs, idempotency keys, and Envoy request-ID policy are unchanged.
