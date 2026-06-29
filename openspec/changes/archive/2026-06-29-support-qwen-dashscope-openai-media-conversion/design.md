## Context

The ai-proxy Qwen provider already recognizes OpenAI API names for chat, embeddings, image generation, and audio speech at the shared router level, and it can route configured capabilities through the provider capability map. Qwen-specific body conversion currently covers chat completions and embeddings in DashScope native mode, while image generation and audio speech fall back to default handling and fail before DashScope receives a valid native payload.

The companion Modelfusion Console change already preserves explicit `openai/v1/imagegeneration` and `openai/v1/audiospeech` capability mappings and intentionally does not synthesize those capabilities from provider type alone. This plugin change should satisfy that runtime prerequisite without changing Console-side compatibility policy.

Current DashScope docs checked on 2026-06-14 show that Qwen-Image, Wan image-generation models, and Qwen-TTS non-streaming synthesis can use `/api/v1/services/aigc/multimodal-generation/generation`. Qwen-Image and Wan image responses return generated image URLs under `output.choices[].message.content[]`; Qwen-TTS non-streaming responses expose audio metadata through `output.audio`, including the downloadable `url` when successful. The implementation should rely on existing model mapping to select the concrete DashScope model rather than hard-coding one model family.

## Goals / Non-Goals

**Goals:**

- Convert OpenAI-compatible image generation request JSON into DashScope synchronous multimodal-generation request JSON for explicitly configured DashScope-native Qwen providers.
- Convert DashScope successful image generation responses into OpenAI-style image generation JSON using existing ai-proxy response models.
- Convert OpenAI-compatible audio speech request JSON into DashScope Qwen-TTS multimodal-generation request JSON for explicitly configured DashScope-native Qwen providers.
- Convert DashScope successful Qwen-TTS responses into a documented JSON success response that includes the returned audio URL, audio id, expiry, and usage when present.
- Preserve existing Qwen chat, embeddings, rerank, responses, conversations, async task, auth, domain, model mapping, failover, retry, and compatible-mode behavior.
- Keep image/audio support opt-in through explicit capabilities.

**Non-Goals:**

- Do not infer `openai/v1/imagegeneration` or `openai/v1/audiospeech` from Qwen provider type alone.
- Do not add image edit, image variation, video, ASR, realtime speech, or realtime omni conversion.
- Do not add DashScope async task polling for OpenAI image generation.
- Do not implement strict OpenAI audio binary relay in the first pass. That would require fetching the DashScope result URL or streaming base64 audio chunks from a follow-up callout path.
- Do not change public Console/UserWeb compatibility metadata in this repository.

## Decisions

1. Use explicit capabilities as the media feature gate.

   Qwen default capabilities remain limited to APIs the provider already supports by default. Media conversion runs only when the active provider config includes `openai/v1/imagegeneration` or `openai/v1/audiospeech`, preserving the existing projection contract and avoiding accidental routing of unsupported media models.

   Alternative considered: add media paths to Qwen default capabilities. That would make every Qwen provider appear media-capable and would conflict with the companion Console/SDK behavior that requires explicit media metadata.

2. Reuse the DashScope multimodal-generation endpoint for synchronous media.

   Image generation uses the synchronous Qwen-Image/Wan multimodal request shape, with the concrete model chosen through the existing `modelMapping` flow:

   ```json
   {
     "model": "<mapped-image-model>",
     "input": {
       "messages": [
         {
           "role": "user",
           "content": [
             {"text": "<prompt>"}
           ]
         }
       ]
     },
     "parameters": {
       "n": 1,
       "size": "1280*1280"
     }
   }
   ```

   Examples of suitable mapped image models include `qwen-image-2.0-pro`, `qwen-image-max`, `wan2.6-image`, or another synchronous DashScope media model supported by the target region and API key. The converter should not infer the image model from the Qwen provider type.

   Audio speech uses the Qwen-TTS non-streaming request shape, again with the concrete model supplied by model mapping:

   ```json
   {
     "model": "<mapped-tts-model>",
     "input": {
       "text": "<input>",
       "voice": "<voice>"
     }
   }
   ```

   Alternative considered: use DashScope async image-generation task APIs. That would require task polling or returning task ids through a non-OpenAI response contract, which does not satisfy a single-call local E2E media check.

3. Map only fields that are safe and directly representable.

   Image conversion maps `model`, `prompt`, `n`, `size`, and `seed` where supported. `size` is normalized from OpenAI `WIDTHxHEIGHT` to DashScope `WIDTH*HEIGHT`. Unsupported OpenAI image fields are ignored unless a later implementation can map them safely.

   Audio conversion maps `model`, `input`, and `voice`. Qwen-TTS extension fields such as `language_type`, `instructions`, and `optimize_instructions` can be accepted when present because DashScope supports them. OpenAI `response_format` and `speed` are not forced into DashScope native payloads unless a safe model-specific mapping exists.

   Alternative considered: forward all unknown fields. That risks sending invalid native DashScope payloads and makes provider validation errors harder to diagnose.

4. Return success JSON for audio instead of binary relay in this change.

   The first implementation returns a documented JSON object containing the DashScope audio URL and metadata. This proves the OpenAI-style gateway request reaches DashScope and succeeds, while keeping the Wasm response-body flow bounded to the existing transform model. Strict OpenAI SDK binary compatibility remains a separate follow-up because it needs a second outbound fetch or streaming relay.

   Alternative considered: fetch the DashScope audio URL inside `TransformResponseBody` and replace the response with audio bytes. That is more compatible for OpenAI clients but introduces asynchronous response-body callout complexity, binary headers, timeout handling, and larger gateway memory pressure.

5. Keep Qwen compatible mode pass-through unchanged.

   `qwenEnableCompatible=true` continues to preserve upstream compatible-mode behavior. Media conversion is for DashScope-native Qwen provider configurations using explicit capability mappings. If a compatible-mode deployment explicitly configures media capabilities, the request should remain OpenAI-compatible pass-through to the configured upstream path rather than being rewritten to DashScope native JSON.

   Alternative considered: apply media conversion in compatible mode as well. That could break existing compatible-mode passthrough semantics and should only be added if DashScope compatible-mode media endpoints require it.

6. Keep shared request routing, support checks, and model mapping unchanged.

   GitNexus impact analysis shows the shared request-body flow is a critical blast radius: `ProviderConfig.handleRequestBody` has 32 direct provider callers and 18 affected execution flows, `ProviderConfig.isSupportedAPI` has 35 direct callers and 21 affected flows, and `getMappedModel` has 65 impacted symbols across 25 flows. This change should add Qwen-local media helper methods and switch branches rather than changing those shared helpers.

   Alternative considered: extend shared media handling in `ProviderConfig.defaultTransformRequestBody`. That would reach many providers that intentionally pass through OpenAI-compatible media APIs and would create unnecessary cross-provider regression risk.

## Code Impact Analysis

GitNexus and graphify analysis point to the following implementation boundary:

- LOW risk Qwen-local extension points: `qwenProvider.TransformRequestBodyHeaders`, `qwenProvider.TransformResponseBody`, `qwenProvider.GetApiName`, `qwenProviderInitializer.DefaultCapabilities`, `qwenProvider.onChatCompletionRequestBody`, and `qwenProvider.onEmbeddingsRequestBody`.
- CRITICAL risk shared helpers to avoid unless a separate change is approved: `ProviderConfig.handleRequestBody`, `ProviderConfig.isSupportedAPI`, and `getMappedModel`.
- Test impact: `extensions/ai-proxy/provider/qwen_test.go` for direct converter and DTO behavior, `extensions/ai-proxy/test/qwen.go` for proxy-host integration paths, and existing Qwen chat/embedding/compatible-mode cases for regressions.
- Documentation impact: `extensions/ai-proxy/README*.md`, `resources/plugins/ai-proxy/README*.md`, and local provider API docs. The repo already includes Qwen image docs, but Qwen-TTS docs may need to be added or cross-referenced from official Aliyun documentation.

## Risks / Trade-offs

- Audio response is not strict OpenAI binary output -> document the JSON response shape and keep binary relay as a follow-up if required by clients.
- Model/version endpoint compatibility may vary by DashScope model and region -> tests cover transformation shape, and live E2E should use a synchronous multimodal-generation model supported by the configured region and API key.
- Unsupported fields may be silently ignored -> implementation should keep the mapping narrow and tests should assert ignored fields do not corrupt native payloads.
- Capability misconfiguration can still route media requests to unsupported models -> explicit capability checks prevent default synthesis, while upstream errors remain visible as provider errors.
- Shared helper changes could regress many providers -> keep conversion logic Qwen-local and rerun impact analysis if implementation discovers a need to edit shared helpers.
- Response body conversion can mask upstream error details -> DashScope error bodies should remain parseable or return a clear transform error without leaking credentials.

## Migration Plan

1. Create this OpenSpec change and validate it before implementation.
2. Add failing Qwen provider tests for image/audio request conversion and response conversion.
3. Add ai-proxy integration tests covering explicit media capabilities and missing-capability rejection.
4. Implement Qwen media request/response conversion behind explicit capabilities without modifying shared routing, support-check, or model-mapping helpers.
5. Update ai-proxy documentation with Qwen/DashScope media capability examples, supported synchronous media model guidance, and the audio JSON limitation.
6. Run focused Go tests, `openspec validate support-qwen-dashscope-openai-media-conversion --strict`, `git diff --check`, `graphify update .` when code/docs are changed, and `gitnexus detect_changes`.

Rollback is limited to the ai-proxy plugin change. Existing configurations without explicit media capabilities are unaffected, and configurations with explicit media capabilities will return to the previous unsupported behavior if the plugin is rolled back.

## Open Questions

- Should a later follow-up add strict OpenAI `/v1/audio/speech` binary relay by fetching DashScope audio URLs and returning audio bytes?
- Should compatible-mode DashScope media endpoints be supported separately if Aliyun exposes native OpenAI-compatible media APIs for the same models?
