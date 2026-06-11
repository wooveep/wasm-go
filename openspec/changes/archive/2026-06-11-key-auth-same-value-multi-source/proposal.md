## Why

`key-auth` currently rejects requests that present more than one non-empty credential candidate across configured headers and query parameters, even when every candidate normalizes to the same API key. This blocks clients and upstream platforms that send the same key through multiple accepted sources, such as `Authorization: Bearer <api-key>` and `x-api-key: <api-key>`, while still leaving the authenticated identity unambiguous.

## What Changes

- Accept requests that present the same normalized API key value through multiple configured sources.
- Continue rejecting requests that present two or more distinct normalized API key values with the existing multiple-key error behavior.
- Preserve `Authorization: Bearer <api-key>` normalization only for the configured `Authorization` source.
- Preserve consumer-level `keys`, `in_header`, and `in_query` source isolation when choosing an allowed candidate for the authenticated identity.
- Update key-auth documentation and response wording to describe "multiple distinct API keys" instead of rejecting all duplicate source presentations.
- No new configuration fields are introduced.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `key-auth-local-yaml`: Combined header and query extraction will reject multiple distinct normalized credential values while accepting duplicate presentations of the same normalized credential value.

## Impact

- Affected code: `extensions/key-auth/main.go` credential candidate handling in request authentication.
- Affected tests: `extensions/key-auth/main_test.go` local YAML and request-header authentication coverage.
- Affected specs: `openspec/specs/key-auth-local-yaml/spec.md` combined extraction semantics.
- Affected docs: `extensions/key-auth/README.md` and `extensions/key-auth/README_EN.md`.
- Dependencies and configuration schema are unchanged.
