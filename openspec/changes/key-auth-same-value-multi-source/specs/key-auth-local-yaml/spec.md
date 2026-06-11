## MODIFIED Requirements

### Requirement: Combined header and query extraction

`key-auth` SHALL check all enabled credential sources and SHALL preserve multi-credential rejection when more than one distinct normalized credential value is presented.

#### Scenario: Header source authenticates
- **WHEN** `in_header` is enabled and a request presents one valid credential in a configured header key
- **THEN** the request SHALL authenticate with that credential

#### Scenario: Query source authenticates
- **WHEN** `in_query` is enabled and a request presents one valid credential in a configured query key
- **THEN** the request SHALL authenticate with that credential

#### Scenario: Header and query both enabled
- **WHEN** both `in_header` and `in_query` are enabled and a request presents one valid credential in query while configured headers are absent
- **THEN** the request SHALL authenticate with the query credential

#### Scenario: Same normalized credential in multiple sources authenticates
- **WHEN** a request presents the same non-empty normalized credential value through more than one configured header or query source
- **THEN** the request SHALL authenticate as a single credential presentation

#### Scenario: Repeated query parameter with same credential authenticates
- **WHEN** a request presents the same non-empty credential value more than once for one configured query key
- **THEN** the request SHALL authenticate as a single credential presentation

#### Scenario: Multiple distinct presented credentials are rejected
- **WHEN** a request presents more than one distinct non-empty normalized credential value across configured headers and query parameters
- **THEN** the plugin SHALL reject the request as multiple key authentication data

#### Scenario: Authorization Bearer normalization participates in equality comparison
- **WHEN** `Authorization` is configured as a key source and a request presents `Authorization: Bearer real-api-key` and another configured source presents `real-api-key`
- **THEN** the plugin SHALL treat both candidates as the same normalized credential value

#### Scenario: Non-Authorization Bearer value remains distinct
- **WHEN** `Authorization` is configured as a key source, another configured non-Authorization header presents `Bearer real-api-key`, and `Authorization` presents `Bearer real-api-key`
- **THEN** the plugin SHALL treat the Authorization candidate as `real-api-key` and the non-Authorization candidate as `Bearer real-api-key`

#### Scenario: Same-value duplicates respect consumer extraction settings
- **WHEN** a request presents one configured consumer credential through multiple same-value sources and at least one same-value candidate is allowed by that consumer's resolved extraction settings
- **THEN** the request SHALL authenticate as that consumer through an allowed candidate

#### Scenario: Same-value duplicates without an allowed candidate are rejected
- **WHEN** a request presents one configured consumer credential only through same-value sources that are not allowed by that consumer's resolved extraction settings
- **THEN** the request SHALL NOT authenticate as that consumer
