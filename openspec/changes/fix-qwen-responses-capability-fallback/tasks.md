## 1. Capability Lookup

- [x] 1.1 Add omitted, non-empty, empty, and whitespace-only Qwen Responses capability tests.
- [x] 1.2 Make `provider.isSupportedAPI` treat an explicitly empty or whitespace-only Qwen Responses path as unsupported while preserving other providers' empty-path semantics.

## 2. Qwen Responses Fallback

- [x] 2.1 Add Qwen compatible non-streaming Responses fallback coverage.
- [x] 2.2 Add Qwen compatible streaming request and SSE fallback coverage.
- [x] 2.3 Update Chinese and English capability documentation and examples.

## 3. Verification

- [x] 3.1 Run Go formatting and focused/full ai-proxy tests.
- [x] 3.2 Run strict OpenSpec validation, Graphify update, GitNexus change analysis, and diff checks.
- [x] 3.3 Build the `ai-proxy` Wasm artifact for local deployment.
