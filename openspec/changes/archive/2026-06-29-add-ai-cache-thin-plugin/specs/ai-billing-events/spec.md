## ADDED Requirements

### Requirement: Billing events include upstream invocation facts
`ai-billing` SHALL consume trusted gateway facts from cache replay and SHALL emit whether the upstream provider was invoked for the completed AI request.

#### Scenario: Cache replay emits upstream not invoked
- **WHEN** an enabled AI request completes from a trusted `ai-cache` replay
- **AND** the trusted gateway fact indicates the upstream provider was not invoked
- **THEN** `ai-billing` SHALL emit `upstream_invoked` as `false`
- **AND** provider cost settlement consumers SHALL be able to distinguish the event from an upstream provider invocation

#### Scenario: Upstream request emits upstream invoked
- **WHEN** an enabled AI request is forwarded to the upstream provider
- **THEN** `ai-billing` SHALL emit `upstream_invoked` as `true`

#### Scenario: User-supplied fields cannot spoof upstream invocation
- **WHEN** a request or response body contains user-supplied fields named like `upstream_invoked`, cache hit, or billing metadata
- **THEN** `ai-billing` SHALL ignore those user-supplied fields for upstream invocation status
- **AND** it SHALL rely only on trusted gateway-controlled facts

#### Scenario: Cache billing facts are not exposed in user response body
- **WHEN** `ai-cache` replays a cache hit and `ai-billing` observes the response
- **THEN** the user response body SHALL remain OpenAI-compatible
- **AND** `upstream_invoked` SHALL be emitted only in the billing event or trusted internal diagnostics
