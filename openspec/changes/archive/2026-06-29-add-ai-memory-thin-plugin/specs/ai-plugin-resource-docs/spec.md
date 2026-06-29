## ADDED Requirements

### Requirement: ai-memory resource package is generated
The resource catalog SHALL include a complete `ai-memory` resource package that reflects the thin gateway plugin runtime contract.

#### Scenario: ai-memory resource files exist
- **WHEN** the resource catalog is inspected
- **THEN** `resources/plugins/ai-memory/README.md`, `resources/plugins/ai-memory/README_EN.md`, and `resources/plugins/ai-memory/spec.yaml` SHALL exist

#### Scenario: ai-memory documentation explains plugin responsibilities
- **WHEN** an operator reads the `ai-memory` resource documentation
- **THEN** it SHALL explain request gating, tenant and consumer identity extraction, Redis recent-memory fallback, optional Console assemble, request body memory injection, response capture, Redis Stream `MemoryEvent` emission, fail-open behavior, and plugin ordering

#### Scenario: ai-memory documentation explains Console ownership
- **WHEN** an operator reads the `ai-memory` resource documentation
- **THEN** it SHALL state that Console owns PostgreSQL persistence, recent-window materialization, daily digests, semantic recall, embeddings, vector indexing, deletion, repair, management APIs, and backend model cost attribution

#### Scenario: ai-memory documentation explains privacy boundaries
- **WHEN** an operator reads the `ai-memory` resource documentation
- **THEN** it SHALL state that the gateway plugin does not log raw prompts, raw answers, credentials, authorization headers, Redis passwords, or internal bearer tokens

### Requirement: ai-memory resource schema matches plugin configuration
The resource catalog `spec.yaml` for `ai-memory` SHALL expose the thin-plugin configuration fields, inherited route behavior, examples, and secret placeholders.

#### Scenario: Global config fields are present in schema
- **WHEN** `resources/plugins/ai-memory/spec.yaml` is inspected
- **THEN** its global config schema SHALL include `redis_stream`, `recent_cache`, `console_internal`, `tenant_header`, `consumer_header`, `session_header`, `request_id_header`, `enable_path_suffixes`, and `fail_policy`

#### Scenario: Route config fields are present in schema
- **WHEN** `resources/plugins/ai-memory/spec.yaml` is inspected
- **THEN** its route config schema SHALL include `memory_mode`, `recent_window_turns`, `memory_token_budget`, `assemble_timeout_ms`, `inject_role`, `semantic_top_k`, `capture_response`, `no_store_header`, `question_from`, `response_value_from`, `stream_value_from`, and `tool_calls_from`

#### Scenario: Route config schema excludes external targets
- **WHEN** `resources/plugins/ai-memory/spec.yaml` is inspected
- **THEN** its route config schema SHALL NOT allow `redis_stream`, `recent_cache`, or `console_internal` overrides in the first version

#### Scenario: Secret examples use placeholders
- **WHEN** `resources/plugins/ai-memory/spec.yaml` examples are inspected
- **THEN** every internal credential, Redis password, bearer token, or service auth example value SHALL use a placeholder rather than a real secret

#### Scenario: Example shows inherited config model
- **WHEN** `resources/plugins/ai-memory/spec.yaml` examples are inspected
- **THEN** they SHALL show shared Redis Stream, recent-memory Redis, Console internal service, identity headers, enabled AI path suffixes, and `fail_policy: open` in `defaultConfig`, while `matchRules[].config` contains route memory behavior such as `memory_mode`, budgets, capture behavior, and extraction paths
