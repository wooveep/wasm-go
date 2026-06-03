## Context

`ai-billing` is initialized with `wrapper.ParseConfig(parseConfig)`, so the same parser is used for both global config and matched rule config. The current parser treats `billing_service` as mandatory, which makes partial `matchRules[].config` invalid even when `defaultConfig.billing_service` is present.

The wrapper already exposes `ParseOverrideConfig`, which accepts separate global and rule parsers and passes the parsed global config into rule parsing. The matcher tests cover this inheritance pattern, so this change can use the existing wrapper model instead of adding a new matcher abstraction.

`provider` is already captured at request start and emitted through the billing event provider fact. `quota_scope` is parsed into `BillingConfig`, but the runtime billing event currently has no quota-scope field, so the implementation must either wire it into the event or remove it from the documented runtime contract. This change wires it into the event.

## Goals / Non-Goals

**Goals:**

- Parse global `ai-billing` config as a complete config with required `billing_service`.
- Parse rule config as a partial override that inherits the resolved global `BillingConfig`.
- Allow rule-level overrides for `provider`, `quota_scope`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy`.
- Keep full rule configs with `billing_service` backward compatible.
- Emit billing events with the provider and quota scope selected by the matched rule.
- Update plugin examples and resource documentation to recommend global defaults plus rule diffs.

**Non-Goals:**

- Add fail-closed billing delivery behavior. `fail_policy` remains limited to `open`.
- Change billing-service API ownership for settlement, reconciliation, or account balances.
- Change `ai-quota` parsing or monetary quota behavior.

## Decisions

### Use `wrapper.ParseOverrideConfig` for `ai-billing`

`init` will replace `wrapper.ParseConfig(parseConfig)` with `wrapper.ParseOverrideConfig(parseConfig, parseRuleConfig)`.

Rationale: the wrapper already owns global/rule config resolution and provides the global config to the rule parser. Reusing it keeps the behavior aligned with existing matcher infrastructure.

Alternative considered: keep `ParseConfig` and make `parseConfig` detect whether it is parsing a rule. That would require context the parser does not currently receive and would mix global validation with override merging.

### Split full parsing from override overlay

The global parser will continue to validate and populate a complete `BillingConfig`, including `billing_service` and `httpClient`. A new rule parser will begin with a copy of the global config, then apply only fields that exist in `matchRules[].config`.

`billing_service` in rule config remains optional. If present, it is parsed with the same validation as global config and creates a rule-specific HTTP client. If absent, the inherited `BillingService` and inherited client are reused.

Rationale: starting from a fully parsed global config preserves defaults, avoids zero-value regressions, and keeps existing full rule configs valid.

Alternative considered: parse rule configs into a separate partial struct and merge later. That adds a second config model without meaningful benefit for this scoped change.

### Treat field presence as the override signal

Rule parsing will check `gjson.Result.Exists()` for each overrideable field. Empty strings keep the existing defaulting behavior when provided globally; for rule overrides, an empty string will resolve through the same `stringDefault` defaults as current full configs. Arrays such as `enable_path_suffixes` override only when the field exists, so an omitted field inherits global suffixes.

Rationale: this matches operator intent for partial config and keeps validation behavior close to the current parser.

Alternative considered: use pointer fields in `BillingConfig`. That would leak partial-config concerns into runtime request handling and tests.

### Emit quota scope in billing events

`BillingEvent` will gain a quota-scope field, and request handling will capture the matched config's `QuotaScope` along with provider. `buildBillingEvent` will populate the event with the resolved quota scope.

Rationale: existing specs and configuration expose `quota_scope`; without event output it has no runtime effect in `ai-billing`. Emitting it makes rule-level quota-scope overrides observable by billing-service.

Alternative considered: mark `quota_scope` as reserved. That would preserve schema shape but leave route-level billing events unable to communicate the configured scope.

## Risks / Trade-offs

- Rule parser accidentally resets inherited fields to defaults -> Mitigation: test global-only config, partial rule config, path suffix override, fail-policy override, and full rule config compatibility.
- Inherited `httpClient` may be copied by value -> Mitigation: keep the client as the existing interface value and only replace it when a rule-level `billing_service` is explicitly provided.
- Event payload shape changes by adding quota scope -> Mitigation: add the field as an additive JSON property and keep all existing event fields unchanged.
- Missing global `billing_service` must still fail startup -> Mitigation: keep full global parser validation and add an explicit failure test for absent global billing-service.

## Migration Plan

1. Deploy the plugin with unchanged existing full-rule configs; they should continue to parse.
2. Operators can migrate repeated `billing_service` and shared fields into `defaultConfig`.
3. Operators can reduce `matchRules[].config` to only route-specific fields such as `provider`, `quota_scope`, `enable_path_suffixes`, or `fail_policy`.
4. Rollback is configuration-compatible for full-rule configs. Partial rule configs require the new parser and must be expanded before rolling back to an older plugin version.

## Open Questions

None.
