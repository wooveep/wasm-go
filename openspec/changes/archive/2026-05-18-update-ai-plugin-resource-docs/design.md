## Context

`ai-quota` was refactored to monetary quota behavior in the current codebase, but the resource catalog files under `resources/plugins/ai-quota` still describe the previous token quota model and gateway-hosted quota management APIs. Those resource files are user-facing package metadata, so stale schemas can lead operators to configure fields that the plugin no longer uses.

`ai-billing` now exists under `extensions/ai-billing` with README files and a `plugin.yaml` example, but there is no matching resource catalog package under `resources/plugins/ai-billing`. That makes `ai-billing` unavailable or undocumented wherever the resource catalog is used as the plugin discovery source.

The implementation should treat extension documentation and extension `plugin.yaml` files as the source of truth for resource catalog content, while preserving the resource catalog format used by `resources/plugins/*`.

## Goals / Non-Goals

**Goals:**

- Update `resources/plugins/ai-quota` to document the current monetary balance admission and deduction model.
- Generate `resources/plugins/ai-billing` with Chinese and English documentation plus a resource `spec.yaml`.
- Keep resource schemas aligned with the actual config fields accepted by `extensions/ai-quota` and `extensions/ai-billing`.
- Clearly communicate that account, price, balance, billing statement, idempotency, and reconciliation ownership belongs to Console or billing-service rather than gateway plugins.
- Preserve bilingual resource documentation and `x-title-i18n` / `x-description-i18n` metadata where applicable.

**Non-Goals:**

- No Go runtime changes to `ai-quota`, `ai-billing`, `ai-statistics`, `pkg/tokenusage`, or wrapper APIs.
- No changes to Redis key behavior, HTTP callout behavior, billing-service APIs, or monetary cost calculation.
- No migration of the already completed `add-ai-billing-monetary-quota` change.
- No broad documentation rewrite outside the two resource plugin packages unless needed to keep links correct.

## Decisions

1. Use extension README files as the narrative source for resource README files.

   The extension docs already describe the implemented behavior and are closer to the plugin code. Resource docs should mirror that content with only catalog-specific wording changes. This avoids having two independent descriptions of `ai-quota` monetary quota or `ai-billing` event delivery.

2. Use extension `plugin.yaml` examples as the source for resource `spec.yaml` examples and priority values.

   `extensions/ai-quota/plugin.yaml` currently demonstrates monetary quota default and match-rule config. `extensions/ai-billing/plugin.yaml` demonstrates billing-service callout config. Resource examples should not invent separate defaults or priorities.

   For `ai-billing`, the default/example billing-service address is `modelfusion-console.higress-system.svc.cluster.local`.

3. Replace obsolete `ai-quota` schema fields instead of keeping compatibility documentation.

   The current plugin no longer requires or handles `admin_consumer`, `admin_path`, or `redis_key_prefix`, and it no longer exposes `/quota`, `/quota/refresh`, or `/quota/delta`. Keeping these fields in resource metadata would create invalid operator guidance.

4. Model `ai-billing` as an independent resource package.

   `ai-billing` has independent runtime responsibilities and can be deployed without `ai-quota` or `ai-statistics`. The resource package should therefore have its own `README.md`, `README_EN.md`, and `spec.yaml` rather than being described only inside `ai-quota` docs.

5. Validate YAML structure and documentation drift through focused checks.

   Implementation should parse the changed `spec.yaml` files and run targeted searches to confirm that removed `ai-quota` admin terms do not remain in resource docs except as explicit removal notes.

6. Keep resource `readmeUrl` values pointing at extension README files.

   The existing catalog convention points plugin metadata to `plugins/wasm-go/extensions/<plugin>/README*.md`. The generated resource README files mirror extension docs for catalog packaging, but the public GitHub links should remain stable with the extension source.

7. Keep resource `info.version` at `1.0.0`.

   This change corrects resource documentation and schema metadata without changing the plugin runtime contract or publishing a new plugin artifact version.

## Risks / Trade-offs

- Resource schema can still drift from Go config structs over time -> Mitigation: make tasks require comparing resource fields against extension README, extension `plugin.yaml`, and current config parsing.
- Copying extension README content verbatim may miss catalog-specific schema details -> Mitigation: include field tables and examples in resource docs, not only high-level summaries.
- `spec.yaml` catalog conventions are only represented by existing resource files in this repository -> Mitigation: preserve the current resource file shape and only change plugin-specific metadata, schema properties, examples, and docs.
- `ai-quota` removal notes could be confused with supported admin APIs -> Mitigation: word them explicitly as removed/unsupported behavior and exclude obsolete fields from schemas and examples.

## Migration Plan

1. Update `resources/plugins/ai-quota` docs and schema in place.
2. Add `resources/plugins/ai-billing` docs and schema as a new resource package.
3. Validate both resource `spec.yaml` files parse as YAML.
4. Search the resource docs for obsolete `ai-quota` fields and admin endpoints to confirm they only appear in removal statements, if at all.
5. Run `openspec validate update-ai-plugin-resource-docs --strict`.
6. Run `graphify update .` after modifying docs to refresh the project graph.

Rollback is straightforward because this change only affects documentation and resource metadata: revert the changed files under `resources/plugins` and the OpenSpec change artifacts.

## Open Questions

None.
