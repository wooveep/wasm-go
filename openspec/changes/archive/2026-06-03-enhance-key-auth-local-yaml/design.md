## Context

The Go `key-auth` plugin currently parses one credential per consumer into a `credential -> name` map and extracts credentials from global `keys` using global `in_header` / `in_query` settings. It then authenticates the credential, optionally checks route/domain `allow`, and injects `X-Mse-Consumer`.

The C++ `key_auth` plugin supports a richer local configuration model: top-level `credentials`, multiple credentials per consumer, consumer-level extraction settings, `realm`, and fallback key sources such as `X-HI-ORIGINAL-AUTH`. The immediate need is to align the Go plugin with the practical YAML-driven use cases without introducing Redis or another external credential store.

The design is constrained by existing Wasm plugin behavior:

- credential lookup must remain local and fast after config parsing;
- existing single-credential consumer configs must continue to work;
- `global_auth` and route/domain `allow` semantics must not be regressed;
- downstream AI quota/billing plugins rely on identity headers rather than raw API keys;
- implementation must avoid trusting incoming client-supplied `X-Mse-Consumer` or `X-Mse-Tenant` headers.

Before implementation, GitNexus impact analysis must be attempted for each edited Go symbol. The current GitNexus index previously did not expose `extensions/key-auth/main.go` symbols, so implementation should either refresh the index or record the indexing limitation and keep edits scoped.

## Goals / Non-Goals

**Goals:**

- Extend the Go `key-auth` configuration model to support local YAML multi-credential consumers.
- Preserve the existing `credential` field while adding `credentials`.
- Add consumer-level `keys`, `in_header`, and `in_query` overrides with global fallback.
- Add top-level `credentials` authentication-only mode.
- Add `realm` for `WWW-Authenticate` response generation.
- Add Bearer-aware extraction for configured `Authorization` key sources.
- Add optional consumer `tenant` and inject trusted `X-Mse-Tenant` with `X-Mse-Consumer`.
- Keep all credential lookup local and in-memory.
- Update key-auth tests and documentation to describe the new contract.

**Non-Goals:**

- Add Redis, database, HTTP callouts, background refresh, or any dynamic credential source.
- Add API key lifecycle management, rotation APIs, admin endpoints, audit logging, or console integration.
- Hash, encrypt, or redact YAML credentials beyond existing configuration handling.
- Change unrelated authentication plugins.
- Change the public meaning of `global_auth` or `allow`.
- Require `tenant` for existing consumers.

## Decisions

1. Use a normalized local identity map.

   The parser will build an internal credential lookup structure that maps each configured credential to an identity record:

   ```text
   credential -> {
     name:   consumer name, when using consumers mode
     tenant: optional consumer tenant
     mode:   consumer or top-level credentials
   }
   ```

   This keeps request-time lookup O(1) for normal consumer authentication and lets `tenant` propagate without scanning consumers after a credential match.

   Alternative considered: keep `credential2Name` and add separate maps for tenant and mode. That minimizes edits but makes duplicate validation and future extension harder to reason about.

2. Represent consumer credentials as a set while preserving the existing single field.

   `consumers[].credential` remains supported for compatibility. `consumers[].credentials` is added for multiple API keys. A consumer may configure exactly one of those fields. During parsing both forms are normalized into one internal credential set.

   Alternative considered: require only `credentials` going forward. That would simplify the model but would break existing YAML.

3. Make consumer extraction settings override global settings only for that consumer.

   A consumer with `keys` uses only its own key list. A consumer without `keys` uses global `keys`. `in_header` and `in_query` follow the same override pattern, with global values as fallback. Each resolved extraction plan must enable at least one source.

   Alternative considered: merge consumer keys with global keys. That is more permissive but makes per-consumer isolation unclear and diverges from the C++ behavior users expect.

4. Extract from both headers and query parameters when both are enabled.

   The current Go plugin checks query only when header extraction is disabled. This change will collect candidate credentials from all enabled sources, then preserve the existing ambiguous-multiple-credential rejection.

   Alternative considered: keep header precedence over query. That is less invasive but does not match the documented configuration default that both sources can be enabled.

5. Treat `Authorization` Bearer stripping as a key-source behavior.

   When the configured key name is `Authorization` and the header value starts with the standard `Bearer ` prefix, extraction returns only the token after the prefix. Non-Bearer Authorization values remain raw credential values for compatibility.

   Alternative considered: strip `Bearer ` from every header source. That could alter unrelated custom headers and make key matching surprising.

6. Inject authenticated identity headers as trusted values.

   After successful consumer authentication, the plugin will remove or overwrite incoming `X-Mse-Consumer` and `X-Mse-Tenant` before adding authenticated values. `X-Mse-Tenant` is injected only when the matched consumer has `tenant`.

   Alternative considered: add headers without removing existing values. That risks duplicate headers or spoofed identity reaching downstream plugins.

7. Keep top-level `credentials` authentication-only.

   Top-level `credentials` mode validates configured API keys without associating them to a consumer or tenant. It must not inject `X-Mse-Consumer` or `X-Mse-Tenant`, and it must not be used with consumer `allow` authorization.

   Alternative considered: synthesize consumer names from credential values. That would expose or derive identity from secrets and would not provide a stable authorization model.

8. Keep external credential storage out of scope.

   Redis can be revisited later if YAML becomes operationally too large or too frequently rotated. This change should not introduce Redis configuration, Redis wrappers, async callouts, local caches, fail-open/fail-closed storage policies, or new operational dependencies.

   Alternative considered: implement Redis now. That would significantly expand request-path complexity and failure modes before the local YAML behavior is aligned.

## Risks / Trade-offs

- Existing configs unexpectedly fail stricter validation -> preserve old accepted forms and add tests for current examples before tightening only contradictory new forms.
- Header spoofing reaches downstream plugins -> explicitly remove or overwrite `X-Mse-Consumer` and `X-Mse-Tenant` after authentication.
- Multiple key sources create false multi-key failures -> define deterministic candidate collection and keep only non-empty extracted values; document that presenting more than one credential remains invalid.
- `Authorization` casing differences break extraction -> rely on proxy-wasm header lookup behavior and test the casing supported by the test host; avoid custom case-folding that diverges from the host API.
- Top-level `credentials` with `allow` is ambiguous -> treat top-level credentials as authentication-only and avoid injecting consumer identity; validation or documentation must make this boundary clear.
- GitNexus symbol impact may be unavailable for `extensions/key-auth/main.go` -> refresh the index before implementation or record the limitation and compensate with focused tests and code review.
- YAML remains plaintext and may grow large -> document that this phase is local YAML only; future Redis or hashed-key storage requires a separate proposal.

## Migration Plan

1. Add tests that capture existing single-credential, global key, global auth, and route `allow` behavior.
2. Introduce the expanded config model and internal credential identity map while keeping existing YAML valid.
3. Add extraction helpers for combined header/query lookup and Authorization Bearer handling.
4. Add tenant-aware identity header injection with spoofing protection.
5. Add top-level credentials parsing and authentication-only request handling.
6. Update `README.md`, `README_EN.md`, and examples.
7. Run targeted key-auth tests, then broader affected tests if shared helpers are touched.
8. Run `graphify update .` after code changes.
9. Run `gitnexus_detect_changes()` before committing to confirm affected scope.

Rollback is straightforward: revert the key-auth plugin, tests, and docs for this change. No external services, schema migrations, or data migrations are introduced.

## Open Questions

- Whether `Authorization` key matching should accept only exact `Authorization` or also case-insensitive configured variants such as `authorization` depends on current proxy-wasm header behavior and should be verified during implementation.
- Whether top-level `credentials` should be rejected when matched route config contains non-empty `allow` may need a final compatibility decision. The recommended behavior is to keep top-level credentials authentication-only and document that `allow` requires named consumers.
- Whether `X-HI-ORIGINAL-AUTH` fallback should be added in this change or deferred. The requirement is to align practical YAML behavior; if implemented, it should be an additional configured key source/fallback and must not change the Bearer extraction contract.
