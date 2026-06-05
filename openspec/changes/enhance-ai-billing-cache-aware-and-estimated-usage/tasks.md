## 1. Safety and Setup

- [x] 1.1 Run GitNexus upstream impact analysis for `recordUsage`, `onHttpResponseBody`, `onHttpStreamingResponseBody`, and `GetTokenUsage`; report direct callers, affected processes, and risk before editing those symbols.
- [x] 1.2 Inspect the existing `ai-billing` request, non-streaming response, streaming response, and billing delivery flows to identify current context keys and event construction points.
- [x] 1.3 Add `github.com/tiktoken-go/tokenizer` to module dependencies and confirm the selected version exposes `o200k_base` and `cl100k_base` vocabulary support.

## 2. Provider Usage Normalization

- [x] 2.1 Add usage source handling so provider usage emits `usage_source=provider`, estimated usage emits `usage_source=estimated`, and unavailable usage emits `usage_source=missing`.
- [x] 2.2 Preserve complete raw provider usage in `usage.details.provider_usage` for every provider-sourced event.
- [x] 2.3 Extend cache-aware parsing for OpenAI-compatible/GLM, Kimi, DeepSeek, Qwen, Claude/Anthropic, and Gemini usage payloads.
- [x] 2.4 Apply non-negative validation and clamp cached input tokens to provider input tokens before deriving cache hit and miss counts.
- [x] 2.5 Ensure provider usage without supported cache-aware fields emits only basic provider usage plus `details.provider_usage`.

## 3. Tokenizer Estimation

- [ ] 3.1 Implement model-to-vocabulary selection with `o200k_base` fallback for unknown, empty, unmapped, or newly matched models and `cl100k_base` for recognized older OpenAI-compatible models.
- [ ] 3.2 Implement structured input text extraction for Chat Completions `messages[].content`, Responses `input` and `instructions`, and Completions `prompt`.
- [ ] 3.3 Implement non-streaming assistant output text extraction without tokenizing complete raw JSON request bodies when structured extraction fails.
- [ ] 3.4 Implement estimated usage event construction with only `usage.unit`, `usage.input`, `usage.output`, and `usage.total`.
- [ ] 3.5 Implement missing usage fallback with zero token counts, `usage_missing=true`, and `usage_source=missing` when provider usage and estimation are unavailable.

## 4. Streaming Delivery and Output Accounting

- [ ] 4.1 Store request-side text needed for estimation at request time without retaining raw secrets or unnecessary full payloads.
- [ ] 4.2 Accumulate streamed assistant text deltas that have been sent to the client for output estimation.
- [ ] 4.3 Add a request-scoped delivered marker such as `ctxBillingDelivered` and set it before billing event dispatch.
- [ ] 4.4 Keep response body `endOfStream=true` as the normal billing delivery path and make it mark the event as delivered.
- [ ] 4.5 Add stream-done fallback delivery that emits one provider, estimated, or missing billing event only when the delivered marker is not already set.
- [ ] 4.6 Ensure client-interrupted streaming estimates count only text deltas already sent to the client.

## 5. Tests and Verification

- [ ] 5.1 Add unit tests for each supported provider cache-aware field shape and clamping behavior.
- [ ] 5.2 Add unit tests proving provider usage takes precedence over tokenizer estimation and retains raw `provider_usage`.
- [ ] 5.3 Add unit tests for non-streaming estimated usage, unknown-model `o200k_base` fallback, and estimated events excluding cache-aware and provider detail fields.
- [ ] 5.4 Add streaming tests for normal end, missing final usage estimation, client interruption fallback, and duplicate prevention.
- [ ] 5.5 Run focused Go tests for `extensions/ai-billing` and `pkg/tokenusage`, then run any broader suite required by touched shared packages.
- [ ] 5.6 Run `openspec status --change "enhance-ai-billing-cache-aware-and-estimated-usage"` and OpenSpec validation commands available in this repo.
- [ ] 5.7 Run `gitnexus_detect_changes()` before committing to verify affected symbols and flows match the expected blast radius.
- [ ] 5.8 Run `graphify update .` after code changes to keep the project graph current.
