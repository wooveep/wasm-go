## 1. Pre-Change Analysis

- [x] 1.1 Run GitNexus impact analysis for the key-auth request authentication symbols before editing `extensions/key-auth/main.go`, including `extractCredentialCandidates` and `onHttpRequestHeaders`.
- [x] 1.2 Review the current `extensions/key-auth/main.go` candidate extraction, multi-key rejection, credential lookup, and `candidateAllowedByPlan` flow.
- [x] 1.3 Review existing `extensions/key-auth/main_test.go` coverage for header/query extraction, repeated values, Authorization Bearer normalization, top-level `credentials`, consumer source overrides, and authorization errors.

## 2. Core Implementation

- [x] 2.1 Add a small helper or equivalent local logic that groups extracted credential candidates by normalized credential value.
- [x] 2.2 Replace request-time multi-key rejection based on candidate count with rejection based on distinct normalized credential values.
- [x] 2.3 Preserve existing no-key behavior when no normalized credential values are present.
- [x] 2.4 Authenticate the single distinct normalized credential value through the existing credential lookup path.
- [x] 2.5 Select a same-value candidate that satisfies `candidateAllowedByPlan` for the matched identity plan before continuing authenticated request handling.
- [x] 2.6 Preserve existing top-level `credentials`, consumer identity header injection, tenant propagation, `global_auth`, and route/domain `allow` behavior.

## 3. Tests

- [x] 3.1 Add coverage for the same value across `Authorization`, `x-api-key`, and `apikey` headers authenticating one consumer.
- [x] 3.2 Add coverage for the same value across a configured header and query parameter authenticating when both sources are enabled.
- [x] 3.3 Add coverage for different values across configured headers rejecting with `key-auth.multi_key`.
- [x] 3.4 Add coverage for repeated query parameters with the same value authenticating.
- [x] 3.5 Add coverage for repeated query parameters with different values rejecting with `key-auth.multi_key`.
- [x] 3.6 Add coverage proving consumer-level source isolation still applies to same-value duplicates.
- [x] 3.7 Add coverage proving top-level `credentials` mode accepts same-value duplicates without injecting `X-Mse-Consumer` or `X-Mse-Tenant`.
- [x] 3.8 Keep existing no-key, unknown credential, unauthorized consumer, and Bearer normalization tests passing.

## 4. Documentation And Specs

- [x] 4.1 Update `openspec/specs/key-auth-local-yaml/spec.md` at archive time through this change's delta spec.
- [x] 4.2 Update `extensions/key-auth/README.md` to describe same-value multi-source compatibility and "multiple distinct API keys" errors.
- [x] 4.3 Update `extensions/key-auth/README_EN.md` to describe same-value multi-source compatibility and "multiple distinct API keys" errors.
- [x] 4.4 Document that Authorization Bearer normalization participates in equality comparison only for the configured `Authorization` source and that non-Authorization headers are compared as raw values.

## 5. Verification

- [x] 5.1 Run `go test -run 'TestOnHTTPRequestHeaders|TestOnHTTPRequestHeadersLocalYAMLEnhancement' -v ./extensions/key-auth`.
- [x] 5.2 Run `go test ./...`.
- [x] 5.3 Run `openspec validate key-auth-same-value-multi-source --strict` and fix any proposal, design, spec, or task issues.
- [x] 5.4 Run `graphify update .` after code changes to refresh the project graph.
- [x] 5.5 Run `gitnexus_detect_changes()` before committing to verify affected symbols and execution flows are expected.
