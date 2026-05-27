## 1. Pre-Change Analysis

- [x] 1.1 Read `docs/requirements/key-auth-local-yaml-enhancement.md`, this change's `proposal.md`, `design.md`, and `specs/key-auth-local-yaml/spec.md` to confirm scope before editing.
- [x] 1.2 Read `extensions/key-auth/main.go`, `extensions/key-auth/main_test.go`, `extensions/key-auth/README.md`, `extensions/key-auth/README_EN.md`, and `extensions/key-auth/keyauth.yaml` to identify exact edit points.
- [x] 1.3 Run GitNexus impact analysis for each key-auth symbol planned for edits, including `Consumer`, `KeyAuthConfig`, `parseGlobalConfig`, `parseOverrideRuleConfig`, `onHttpRequestHeaders`, `WWWAuthenticateHeader`, and any new helper symbols if GitNexus can resolve them.
- [x] 1.4 If GitNexus cannot resolve `extensions/key-auth/main.go` symbols, run `npx gitnexus analyze` or record the unresolved index limitation in the implementation notes before continuing.
- [x] 1.5 Review proxy-wasm header APIs available in the current dependency to decide whether identity headers should be removed before add, replaced, or otherwise overwritten.

## 2. Configuration Model

- [x] 2.1 Extend the `Consumer` model with optional `Tenant`, `Credentials`, consumer-level `Keys`, and consumer-level source override fields.
- [x] 2.2 Extend `KeyAuthConfig` with top-level `Credentials`, optional `Realm`, and an internal credential identity lookup structure that can represent consumer and top-level credentials.
- [x] 2.3 Preserve existing `consumers[].credential`, global `keys`, global `in_header`, global `in_query`, `global_auth`, and `allow` config fields without requiring YAML changes from existing users.
- [x] 2.4 Keep Redis, database, HTTP callout, dynamic refresh, and remote credential fields out of the key-auth config model.

## 3. Parser Validation

- [x] 3.1 Update global config parsing to reject configs that contain both `consumers` and top-level `credentials` at the same level.
- [x] 3.2 Update parsing to require at least one of `consumers` or top-level `credentials`.
- [x] 3.3 Parse `realm` with default `MSE Gateway` and store it per effective config.
- [x] 3.4 Parse top-level `credentials` as a non-empty authentication-only credential set.
- [x] 3.5 Parse consumer `credential` and `credentials`, requiring exactly one of the two fields for each consumer.
- [x] 3.6 Reject empty consumer `credentials` lists and empty credential strings.
- [x] 3.7 Reject duplicate credential values within one consumer and across different consumers.
- [x] 3.8 Parse optional consumer `tenant` and keep it associated with every normalized credential for that consumer.
- [x] 3.9 Parse consumer-level `keys`, `in_header`, and `in_query` overrides and validate each resolved extraction plan has at least one enabled source.
- [x] 3.10 Validate global `keys` remains required for any top-level credentials config or consumer that does not provide consumer-level `keys`.
- [x] 3.11 Preserve existing route/domain override parsing for `allow`, including current error behavior for missing or empty `allow`.

## 4. Credential Extraction

- [x] 4.1 Add a helper that resolves the effective extraction plan for a consumer or top-level credentials mode.
- [x] 4.2 Add a helper that extracts non-empty credential candidates from all enabled header sources.
- [x] 4.3 Add a helper that extracts non-empty credential candidates from all enabled query sources.
- [x] 4.4 Change request handling so header and query sources are both checked when both are enabled.
- [x] 4.5 Add Authorization-specific Bearer parsing so configured `Authorization` header values starting with `Bearer ` match on the token after the prefix.
- [x] 4.6 Preserve raw non-Bearer `Authorization` values as credential candidates.
- [x] 4.7 Ensure Bearer stripping is not applied to non-Authorization headers.
- [x] 4.8 Preserve existing multi-key rejection when more than one non-empty candidate credential is present.
- [x] 4.9 Verify query parsing still handles repeated query parameter values as multiple candidates.

## 5. Authentication And Authorization Flow

- [x] 5.1 Update consumer-mode authentication to match candidates against the normalized credential identity lookup.
- [x] 5.2 Update top-level credentials mode to authenticate matching credentials without consumer name or tenant identity.
- [x] 5.3 Preserve existing no-key, multi-key, unknown credential, and unauthorized consumer response behavior.
- [x] 5.4 Preserve current `global_auth` behavior when global auth is true, false, or omitted.
- [x] 5.5 Preserve `allow` checks against authenticated consumer `name`.
- [x] 5.6 Ensure top-level credentials mode is not treated as a named consumer for `allow` authorization.
- [x] 5.7 Use configured `realm` for all key-auth `WWW-Authenticate` responses.

## 6. Trusted Identity Header Injection

- [x] 6.1 Add a helper for trusted identity propagation after successful consumer authentication.
- [x] 6.2 Ensure authenticated consumer requests propagate `X-Mse-Consumer` with the configured consumer `name`.
- [x] 6.3 Ensure authenticated consumer requests propagate `X-Mse-Tenant` only when the matched consumer has `tenant`.
- [x] 6.4 Ensure incoming client-supplied `X-Mse-Consumer` cannot override the authenticated consumer value.
- [x] 6.5 Ensure incoming client-supplied `X-Mse-Tenant` cannot override the authenticated tenant value.
- [x] 6.6 Ensure top-level credentials authentication does not inject `X-Mse-Consumer` or `X-Mse-Tenant`.

## 7. Tests

- [x] 7.1 Add or update tests proving existing single-credential consumer configs still parse and authenticate.
- [x] 7.2 Add parser tests for `consumers[].credentials` with multiple credentials.
- [x] 7.3 Add parser tests rejecting consumer configs with both `credential` and `credentials`.
- [x] 7.4 Add parser tests rejecting empty credentials and duplicate credentials.
- [x] 7.5 Add parser tests for optional `tenant` and configured `realm`.
- [x] 7.6 Add parser tests for top-level `credentials` mode and conflict with `consumers`.
- [x] 7.7 Add authentication tests for first and second credentials on the same consumer.
- [x] 7.8 Add authentication tests for consumer-level `keys` overriding global `keys`.
- [x] 7.9 Add authentication tests for consumer fallback to global `keys`.
- [x] 7.10 Add authentication tests for consumer-level `in_header` and `in_query` overrides.
- [x] 7.11 Add authentication tests proving query credentials are checked when both header and query extraction are enabled and no header credential is present.
- [x] 7.12 Add authentication tests for `Authorization: Bearer <api-key>`.
- [x] 7.13 Add authentication tests for raw non-Bearer `Authorization` values.
- [x] 7.14 Add authentication tests proving Bearer stripping is not applied to non-Authorization headers.
- [x] 7.15 Add multi-key rejection tests across headers, query parameters, and repeated query values.
- [x] 7.16 Add allow-list tests for allowed and disallowed named consumers after multi-credential authentication.
- [x] 7.17 Add tests proving top-level credentials authenticate without injecting `X-Mse-Consumer` or `X-Mse-Tenant`.
- [x] 7.18 Add tests proving `X-Mse-Consumer` and `X-Mse-Tenant` spoofed by the incoming request are overwritten or removed in favor of authenticated identity.
- [x] 7.19 Add tests proving configured `realm` appears in failure response headers.

## 8. Documentation And Examples

- [x] 8.1 Update `extensions/key-auth/README.md` to document `credentials`, consumer-level key extraction settings, `tenant`, `realm`, and Authorization Bearer extraction.
- [x] 8.2 Update `extensions/key-auth/README_EN.md` with the same behavior and examples.
- [x] 8.3 Update key-auth examples to show local YAML multi-credential consumers and tenant propagation.
- [x] 8.4 Document that this change keeps credentials in local YAML and does not require Redis or another external credential store.
- [x] 8.5 Document that `X-Mse-Consumer` and `X-Mse-Tenant` are authenticated identity headers and should not be supplied by clients as trusted values.
- [x] 8.6 Update any generated plugin metadata or resource docs if this repo's documentation generation process requires it for key-auth fields.

## 9. Verification

- [x] 9.1 Run targeted key-auth tests from `extensions/key-auth`.
- [x] 9.2 Run formatting for changed Go files.
- [x] 9.3 Run broader affected package tests if shared wrappers, matchers, or test helpers are modified.
- [x] 9.4 Run `openspec status --change enhance-key-auth-local-yaml` and confirm proposal, design, specs, and tasks are complete.
- [x] 9.5 Run `graphify update .` after code changes to refresh the local knowledge graph.
- [x] 9.6 Run `gitnexus_detect_changes()` before committing to confirm affected scope and execution flows match expected key-auth changes.
- [x] 9.7 Review `git diff` to ensure no Redis, database, HTTP callout, or unrelated auth plugin changes were introduced.
- [x] 9.8 Record any unresolved open questions from `design.md` in the implementation summary if they affect behavior.
