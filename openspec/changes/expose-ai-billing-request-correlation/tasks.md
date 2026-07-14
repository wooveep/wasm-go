## 1. Safety gates

- [x] 1.1 Run Graphify query and GitNexus context or impact analysis for `ai-billing` response-header handling
- [x] 1.2 Add failing tests for trusted replacement, streaming correlation, and missing context

## 2. Implementation

- [x] 2.1 Replace the internal gateway request-ID response header from request-scoped billing context
- [x] 2.2 Update `ai-billing` documentation for the internal correlation header

## 3. Verification

- [x] 3.1 Run focused and complete `ai-billing` tests
- [x] 3.2 Update Graphify, run GitNexus change detection, build the Wasm artifact, and verify the uploaded plugin
