## 1. Pre-Change Analysis

- [x] 1.1 Run GitNexus impact analysis for `extensions/ai-billing/main.go:init`, `parseConfig`, request context setup, and event-building symbols before editing them.
- [x] 1.2 Inspect existing `ai-billing` tests and wrapper override parser tests to confirm expected match-rule config format and test harness APIs.

## 2. Parser Implementation

- [ ] 2.1 Switch `ai-billing` initialization from `wrapper.ParseConfig(parseConfig)` to `wrapper.ParseOverrideConfig(parseConfig, parseRuleConfig)`.
- [ ] 2.2 Refactor config parsing helpers so global parsing still requires `billing_service` and constructs the billing-service HTTP client.
- [ ] 2.3 Implement `parseRuleConfig` to copy the parsed global `BillingConfig` and overlay only fields present in the matched rule config.
- [ ] 2.4 Support optional rule-level `billing_service` for backward-compatible full rule configs, including rule-specific HTTP client creation.
- [ ] 2.5 Ensure `provider`, `quota_scope`, `tenant_header`, `consumer_header`, `enable_path_suffixes`, and `fail_policy` inherit global values when omitted and override when explicitly present.

## 3. Billing Event Semantics

- [ ] 3.1 Add resolved quota-scope capture to request context or equivalent runtime state for matched requests.
- [ ] 3.2 Add an additive quota-scope field to `BillingEvent` JSON output.
- [ ] 3.3 Ensure generated billing events contain the provider and quota scope resolved from the matched rule config.

## 4. Tests

- [ ] 4.1 Add a global-only config test that initializes with only `defaultConfig.billing_service` and defaultable fields.
- [ ] 4.2 Add a global plus partial rule test where the rule only configures `provider` and inherits `billing_service`.
- [ ] 4.3 Add request/event tests proving different match rules emit different provider values.
- [ ] 4.4 Add request/event tests proving rule-level `quota_scope` is emitted.
- [ ] 4.5 Add tests for rule-level `fail_policy` and `enable_path_suffixes` override behavior.
- [ ] 4.6 Add a failure test for missing global `billing_service` with a clear error.
- [ ] 4.7 Keep or extend coverage proving existing complete rule configs still parse and run.

## 5. Documentation and Examples

- [ ] 5.1 Update `extensions/ai-billing/plugin.yaml` to show shared `defaultConfig` and partial `matchRules[].config` as the recommended shape.
- [ ] 5.2 Update `resources/plugins/ai-billing/spec.yaml` examples and route config schema text to document partial route overrides.
- [ ] 5.3 Update `extensions/ai-billing` and resource README content to describe config inheritance and quota-scope event semantics.

## 6. Verification

- [ ] 6.1 Run focused `ai-billing` Go tests.
- [ ] 6.2 Run relevant wrapper or matcher tests if parser integration behavior changes or is relied on directly.
- [ ] 6.3 Run `openspec status --change support-ai-billing-global-config-and-rule-overrides` and resolve any artifact issues.
- [ ] 6.4 Run `gitnexus_detect_changes()` before committing to verify affected symbols and execution flows match the intended scope.
- [ ] 6.5 Run `graphify update .` after code changes to refresh the project graph.
