## 1. Governance and Context

- [x] 1.1 Run `openspec validate support-qwen-dashscope-openai-media-conversion --strict` before implementation starts.
- [x] 1.2 Read the accepted OpenSpec specs that apply to ai-proxy behavior and documentation before touching code.
- [x] 1.3 Recheck current DashScope Qwen-Image, Wan image-generation, and Qwen-TTS HTTP docs for endpoint, request, response, region, and model support before finalizing mappings.
- [x] 1.4 Run GitNexus upstream impact analysis before editing each Qwen provider function, method, or model type involved in media conversion.
- [x] 1.5 Treat `ProviderConfig.handleRequestBody`, `ProviderConfig.isSupportedAPI`, and `getMappedModel` as no-change shared helpers unless a separate impact review explicitly approves touching them.
- [x] 1.6 Keep unrelated dirty files such as `AGENTS.md` and `CLAUDE.md` out of the implementation diff.

## 2. Tests First

- [x] 2.1 Add Qwen provider unit tests for OpenAI image generation request conversion into DashScope multimodal-generation JSON.
- [x] 2.2 Add Qwen provider unit tests for image parameter mapping, including model mapping, `n`, `seed`, and `size` normalization from `WIDTHxHEIGHT` to `WIDTH*HEIGHT`.
- [x] 2.3 Add Qwen provider unit tests for DashScope image response conversion into OpenAI-style `created` and `data[].url` JSON.
- [x] 2.4 Add Qwen provider unit tests for OpenAI audio speech request conversion into DashScope Qwen-TTS JSON, including model mapping.
- [x] 2.5 Add Qwen provider unit tests for optional DashScope TTS extension fields such as `language_type`, `instructions`, and `optimize_instructions`.
- [x] 2.6 Add Qwen provider unit tests for DashScope audio response conversion into documented audio URL success JSON.
- [ ] 2.7 Add ai-proxy integration tests proving explicit `openai/v1/imagegeneration` and `openai/v1/audiospeech` capabilities enable DashScope-native media conversion when `qwenEnableCompatible` is disabled.
- [ ] 2.8 Add integration tests proving configured media capability paths are used for upstream routing and Qwen default capabilities do not synthesize image or audio speech support.
- [ ] 2.9 Add regression tests proving missing media capabilities remain unsupported.
- [ ] 2.10 Add regression tests proving existing Qwen chat, embeddings, rerank, async APIs, Anthropic messages, and compatible-mode behavior remain unchanged.
- [ ] 2.11 Add compatible-mode media tests proving explicit media capabilities remain pass-through rather than DashScope-native body conversion.

## 3. Qwen Provider Implementation

- [ ] 3.1 Add Qwen media request and response model structs needed for DashScope image generation and Qwen-TTS conversion.
- [ ] 3.2 Implement Qwen image generation request conversion behind `ApiNameImageGeneration`.
- [ ] 3.3 Implement Qwen image generation response conversion with provider-error handling.
- [ ] 3.4 Implement Qwen audio speech request conversion behind `ApiNameAudioSpeech`.
- [ ] 3.5 Implement Qwen audio speech response conversion with provider-error handling and documented audio URL JSON.
- [ ] 3.6 Ensure request headers, content type, accept headers, authorization, domain override, and configured capability paths are preserved correctly.
- [ ] 3.7 Reuse existing model-mapping behavior for both image and audio model names without changing `getMappedModel`.
- [ ] 3.8 Ensure Qwen compatible mode continues to pass through existing compatible-mode APIs and explicitly configured media APIs unchanged.

## 4. Documentation

- [ ] 4.1 Update ai-proxy README documentation with Qwen/DashScope media capability examples for image generation and audio speech.
- [ ] 4.2 Document supported synchronous DashScope media model expectations and show model-mapping examples for Qwen-Image/Wan image and Qwen-TTS.
- [ ] 4.3 Document the first-pass audio speech response contract, including that strict OpenAI binary audio relay is not implemented in this change.
- [ ] 4.4 Add or refresh local DashScope Qwen-TTS documentation references if they are missing from `docs/AIproviderAPI/qwen/`.
- [ ] 4.5 Update any provider capability examples that mention `openai/v1/imagegeneration` or `openai/v1/audiospeech` so they match the new Qwen behavior.

## 5. Verification

- [ ] 5.1 Run focused Qwen provider Go tests.
- [ ] 5.2 Run focused ai-proxy Go tests covering media conversion and regressions.
- [ ] 5.3 Run `openspec validate support-qwen-dashscope-openai-media-conversion --strict`.
- [ ] 5.4 Run `git diff --check`.
- [ ] 5.5 Run `graphify update .` after code or documentation changes are complete.
- [ ] 5.6 Run `gitnexus detect_changes` before handoff or commit.

## 6. Optional Live Compatibility Check

- [ ] 6.1 Build or package the updated ai-proxy plugin for the local Higress environment when implementation verification requires a live gateway check.
- [ ] 6.2 Run the Modelfusion local E2E media checks against the updated plugin and verify image generation and audio speech no longer report `plugin_conversion_gap`.
- [ ] 6.3 Record sanitized evidence for any remaining upstream provider errors without exposing API keys, temporary URLs, or credentials.
