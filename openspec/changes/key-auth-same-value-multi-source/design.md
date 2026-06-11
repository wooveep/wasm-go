## Context

The Go `key-auth` plugin extracts credential candidates from configured header and query sources, then rejects a request when more than one non-empty candidate is present. That behavior prevents ambiguous authentication, but it is stricter than necessary when every presented candidate normalizes to the same API key value.

Some clients and upstream platforms send the same credential through multiple configured sources, for example `Authorization: Bearer <api-key>` and `x-api-key: <api-key>`. The authenticated identity is unambiguous in that case, but the current request path rejects the request with `key-auth.multi_key`.

The accepted `key-auth-local-yaml` capability already defines local YAML credentials, source extraction, Authorization Bearer normalization, consumer-level source overrides, top-level `credentials`, and allow-list behavior. This change narrows only the request-time multi-key rejection condition.

## Goals / Non-Goals

**Goals:**

- Accept duplicate credential presentations when all candidates have the same normalized credential value.
- Continue rejecting requests that present multiple distinct normalized credential values.
- Preserve existing `Authorization: Bearer <api-key>` normalization only for the configured `Authorization` source.
- Preserve consumer-level `keys`, `in_header`, and `in_query` isolation when matching a same-value candidate to the authenticated identity.
- Preserve top-level `credentials`, route/domain `allow`, `global_auth`, `X-Mse-Consumer`, and `X-Mse-Tenant` behavior.
- Update tests, OpenSpec requirements, and key-auth documentation for the new compatibility behavior.

**Non-Goals:**

- Do not add a configuration flag for this behavior.
- Do not introduce priority-based source selection that silently ignores different credential values.
- Do not strip `Bearer ` from non-Authorization headers.
- Do not change duplicate credential validation in YAML configuration.
- Do not change Consumer identity, tenant propagation, or allow-list semantics.
- Do not change Modelfusion-generated key-auth configuration as part of this plugin change.

## Decisions

### Group candidates by normalized credential value

`extractCredentialCandidates` should continue collecting candidates from all configured plans and sources after existing normalization. Request authentication should group the returned candidates by `candidate.Value` before deciding whether the request contains no key, one key, or multiple keys.

Rationale: this preserves the current extraction boundary and keeps Authorization Bearer normalization in one place. The multi-key decision then operates on credential identity rather than source count.

Alternative considered: select the first credential candidate and ignore later candidates. This is rejected because it can mask distinct credential values and weaken the existing ambiguity protection.

### Reject only multiple distinct normalized values

The request path should map grouped candidates as follows:

- Zero distinct values: existing `key-auth.no_key` behavior.
- One distinct value: authenticate that value.
- Two or more distinct values: existing `key-auth.multi_key` behavior.

Rationale: this preserves security for ambiguous requests while allowing unambiguous duplicate presentations.

Alternative considered: source priority configuration. This is rejected because it adds configuration surface and could silently discard conflicting credentials.

### Authenticate by value, then choose an allowed same-value candidate

After resolving the credential identity by the single normalized value, the plugin must select a candidate from that value group that satisfies `candidateAllowedByPlan` for the matched identity plan.

Rationale: same-value duplicates can include both allowed and disallowed sources. Authentication must not simply use the first candidate, because consumer-level `keys`, `in_header`, and `in_query` overrides remain part of the contract.

Alternative considered: skip candidate-plan validation once the credential value is known. This is rejected because it would let a source outside the matched consumer extraction plan authenticate that consumer.

## Risks / Trade-offs

- Same-value duplicates become accepted by default -> This is intentional compatibility behavior and does not require migration because distinct values remain rejected.
- Grouping logic could accidentally treat raw `Bearer <key>` values in non-Authorization headers as equal to Authorization values -> Tests must verify that Bearer stripping remains limited to the `Authorization` source.
- Candidate selection could bypass consumer source isolation -> Tests must cover same-value duplicates where only one candidate is allowed by the matched identity plan and where no candidate is allowed.
- Documentation could remain ambiguous about "multiple keys" -> README error wording and extraction examples must say "multiple distinct API keys".

## Migration Plan

No configuration migration is required. Existing clients that send exactly one credential are unaffected. Existing clients that send different credentials through multiple sources continue to fail with `key-auth.multi_key`. Clients that send the same normalized credential through multiple configured sources authenticate normally after the plugin update.

Rollback is a code rollback only. There is no persisted data or schema migration.

## Open Questions

None.
