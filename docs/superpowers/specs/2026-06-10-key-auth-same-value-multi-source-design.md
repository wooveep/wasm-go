# key-auth Same-Value Multi-Source Compatibility Design

Date: 2026-06-10
Status: Approved for implementation planning

## Context

The Go `key-auth` plugin currently rejects any request that presents more than one non-empty credential candidate across configured headers and query parameters. The rejection appears as `key-auth.multi_key` in response details.

This behavior is safe but too strict for clients and upstream platforms that send the same API key through more than one configured source, such as both `Authorization: Bearer <api-key>` and `x-api-key: <api-key>`. Modelfusion currently generates key-auth configurations that accept `apikey`, `x-api-key`, and `Authorization` header sources, so duplicate same-value credentials can fail even though the authenticated identity is unambiguous.

The accepted `key-auth-local-yaml` OpenSpec currently says multiple presented credentials are rejected. This change must update that semantic to reject multiple distinct normalized credential values while accepting duplicate presentations of the same normalized credential value.

GitNexus impact checks found the code impact is low:

- `extractCredentialCandidates` is called only by `onHttpRequestHeaders`.
- `onHttpRequestHeaders` is the request authentication boundary for `extensions/key-auth/main.go`.

## Goals

- Allow requests that present the same normalized API key value through multiple configured sources.
- Preserve `key-auth.multi_key` rejection when a request presents two or more distinct normalized API key values.
- Preserve existing `Authorization: Bearer <api-key>` normalization only for the configured `Authorization` source.
- Preserve consumer-level `keys`, `in_header`, and `in_query` isolation.
- Preserve top-level `credentials`, route/domain `allow`, `global_auth`, `X-Mse-Consumer`, and `X-Mse-Tenant` behavior.
- Avoid adding new configuration fields.

## Non-Goals

- Do not introduce priority-based source selection that silently ignores different credential values.
- Do not strip `Bearer ` from non-Authorization headers.
- Do not change duplicate credential validation in YAML configuration.
- Do not change Consumer identity, tenant propagation, or allow-list semantics.
- Do not change Modelfusion-generated key-auth configuration as part of this plugin change.

## Decision

Enable same-value multi-source compatibility by default.

The plugin will continue to extract candidates from all enabled plans and sources. Before deciding whether a request contains multiple keys, it will group candidates by their normalized credential value:

```text
request
  -> extract header/query candidates
  -> normalize Authorization Bearer values
  -> group by candidate.Value

0 distinct values
  -> key-auth.no_key

1 distinct value
  -> authenticate that value
  -> find at least one same-value candidate allowed by the matched identity plan
  -> continue existing allow/global_auth flow

2+ distinct values
  -> key-auth.multi_key
```

Examples:

```text
Authorization: Bearer sk-a
x-api-key: sk-a
apikey: sk-a
=> one normalized value, sk-a; authenticate normally

Authorization: Bearer sk-a
x-api-key: sk-b
=> two normalized values, sk-a and sk-b; reject with key-auth.multi_key

Authorization: Bearer sk-a
x-api-key: Bearer sk-a
=> two normalized values, sk-a and Bearer sk-a; reject with key-auth.multi_key
```

## Architecture

The change should stay within `extensions/key-auth/main.go`.

Recommended internal shape:

- Keep `extractCredentialCandidates` responsible for collecting raw candidate records after existing normalization.
- Add a small helper that reduces candidates to credential-value groups.
- Replace the `len(tokens) > 1` check in `onHttpRequestHeaders` with distinct-value logic.
- After resolving the identity by value, select a candidate from that same-value group that passes `candidateAllowedByPlan`.

The important boundary is that authentication must not simply use the first candidate. Same-value duplicates can include sources that are valid for another consumer plan but not valid for the matched credential identity. The selected candidate must satisfy the matched identity's extraction plan.

## Error Handling

Existing error categories remain:

- No normalized values: `key-auth.no_key`.
- More than one distinct normalized value: `key-auth.multi_key`.
- One normalized value not found in configured credentials: existing unauthorized behavior.
- One normalized value found, but none of its candidates are allowed by the matched identity plan: existing unauthorized behavior.
- Authenticated consumer not allowed by matched route/domain allow-list: existing unauthorized behavior.

The response documentation should change from "multiple API keys" to "multiple distinct API keys" so operators understand same-value duplicates are accepted.

## Tests

Update `extensions/key-auth/main_test.go` to cover:

1. Same value across `Authorization`, `x-api-key`, and `apikey` headers authenticates one consumer.
2. Same value across header and query authenticates when both sources are enabled.
3. Different values across configured headers still reject with `key-auth.multi_key`.
4. Repeated query parameter with the same value authenticates.
5. Repeated query parameter with different values rejects with `key-auth.multi_key`.
6. Consumer-level source isolation still works: same-value duplicates authenticate only when at least one candidate is allowed by the matched identity plan.
7. Top-level `credentials` mode accepts same-value duplicates but still does not inject `X-Mse-Consumer` or `X-Mse-Tenant`.
8. Existing no-key, unknown credential, unauthorized consumer, and Bearer normalization tests still pass.

Focused verification:

```bash
go test -run 'TestOnHTTPRequestHeaders|TestOnHTTPRequestHeadersLocalYAMLEnhancement' -v ./extensions/key-auth
```

Full plugin verification from the `wasm-go` repository root:

```bash
go test ./...
```

## Documentation And Specs

Update the accepted `openspec/specs/key-auth-local-yaml/spec.md` requirement for combined header and query extraction:

- Old behavior: any request with more than one presented credential value is rejected.
- New behavior: any request with more than one distinct normalized credential value is rejected; duplicate presentations of the same normalized credential value are accepted.

Update `extensions/key-auth/README.md` and `extensions/key-auth/README_EN.md`:

- Describe same-value multi-source compatibility.
- Clarify that `Authorization: Bearer <api-key>` normalization is applied before equality comparison.
- Clarify that non-Authorization headers are compared as raw values.
- Update the error table to say "multiple distinct API keys".

## Rollout And Compatibility

No configuration migration is required because the behavior is default-on and narrows only the duplicate-source case.

Existing clients that send exactly one credential are unaffected. Existing clients that send different credentials continue to fail with `key-auth.multi_key`. Clients that send the same normalized credential through multiple configured sources now authenticate normally.

## Open Questions

None. The design intentionally avoids a configuration switch and priority-based source selection.
