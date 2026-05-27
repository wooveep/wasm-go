## Why

The Go `key-auth` plugin currently supports only a narrow local YAML credential model, while the C++ `key_auth` plugin supports richer credential layouts and per-consumer extraction controls. Aligning the practical behavior now lets users authenticate API consumers through local YAML with multiple keys, Bearer Authorization headers, and tenant identity propagation without introducing Redis or another external store.

## What Changes

- Add local YAML support for `consumers[].credentials` while preserving existing `consumers[].credential`.
- Add optional consumer-level `keys`, `in_header`, and `in_query` overrides, with global settings as fallback.
- Add top-level `credentials` mode for authentication-only configurations that do not need consumer identity or tenant propagation.
- Add optional `realm` configuration for `WWW-Authenticate` responses, defaulting to `MSE Gateway`.
- Extract API keys from `Authorization: Bearer <api-key>` when `Authorization` is configured as a key source, while preserving raw non-Bearer Authorization values.
- Add optional `consumers[].tenant` and inject trusted `X-Mse-Tenant` together with `X-Mse-Consumer` after successful consumer authentication.
- Preserve existing `global_auth`, route/domain `allow`, error response, and backward-compatible single-credential behavior.
- Keep all credential lookup local and in-memory after parsing YAML.
- Explicitly keep Redis, database, HTTP callout, and dynamic credential loading out of scope for this change.

## Capabilities

### New Capabilities

- `key-auth-local-yaml`: Defines the local YAML key-auth contract for multi-credential consumers, per-consumer key extraction, Bearer Authorization extraction, tenant propagation, top-level credentials mode, validation, and backward compatibility.

### Modified Capabilities

- None.

## Impact

- Affected plugin code: `extensions/key-auth/main.go` configuration model, parser validation, credential extraction, authentication lookup, `allow` checks, response realm handling, and identity header injection.
- Affected tests: `extensions/key-auth/main_test.go` parsing and request authentication scenarios for new and existing YAML forms.
- Affected docs/examples: `extensions/key-auth/README.md`, `extensions/key-auth/README_EN.md`, and related examples should document multi-credential consumers, Bearer Authorization, tenant propagation, and local YAML scope.
- Affected downstream behavior: downstream plugins can rely on authenticated `X-Mse-Consumer` and optional `X-Mse-Tenant`; incoming spoofed identity headers must not override authenticated identity.
- External systems: no new external service dependency; Redis-backed credential lookup remains a future, separate change.
