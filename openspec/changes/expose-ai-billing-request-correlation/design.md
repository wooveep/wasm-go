## Context

`ai-billing` captures the canonical Envoy `x-request-id` at request start and reuses it in the billing event. Internal relay callers cannot currently learn that value from the response. Preserving a caller-supplied request ID globally would weaken correlation and idempotency trust, so the plugin must expose only its already-captured trusted context value.

## Goals / Non-Goals

**Goals:**

- Expose the exact billing event `request_id` to a trusted relay.
- Replace any provider-supplied value under the internal header name.
- Cover streaming, non-streaming, and local/cache response paths.

**Non-Goals:**

- Change request-ID generation, billing event identity, or idempotency keys.
- Preserve caller-controlled external request IDs.
- Expose credentials, usage, or other billing facts in response headers.

## Decisions

1. In the response-header callback, read the request-scoped canonical ID captured during request headers and call `ReplaceHttpResponseHeader` for `x-ocmf-gateway-request-id`.
2. Replace rather than append so an upstream response cannot create ambiguity or spoof the trusted value.
3. If the request context has no canonical ID, remove any existing header so a provider value cannot pass through; do not invent a second correlation identity.
4. Keep event construction and delivery untouched. Tests will compare the header value to the request ID used by the billing event for normal and streaming lifecycle paths.

## Risks / Trade-offs

- [The response-header callback runs for all billed traffic] → Limit the change to one bounded header replacement and run the complete `ai-billing` suite.
- [Internal metadata becomes visible to the downstream relay] → Use a product-specific header containing only a request UUID and no billing or credential data.
- [Plugin ordering can include local cache responses] → Test the callback independently of upstream response origin.

## Migration Plan

Build and upload a new `ai-billing` plugin image, update the managed binding, and verify the new header before enabling Console consumption. Older plugin versions simply omit the header. Rollback has no data migration.

## Open Questions

None.
