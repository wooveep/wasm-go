## Why

OpenAI-compatible image generation and audio speech requests can be routed to the Qwen/DashScope provider today only when explicit ai-proxy capabilities are projected, but the provider implementation still lacks request and response conversion for those media APIs. This leaves correctly configured `/v1/images/generations` and `/v1/audio/speech` gateway routes failing with unsupported API or request-body conversion errors instead of reaching DashScope successfully.

## What Changes

- Add Qwen/DashScope provider conversion for OpenAI-compatible `/v1/images/generations` requests when `openai/v1/imagegeneration` is explicitly configured.
- Add Qwen/DashScope provider conversion for OpenAI-compatible `/v1/audio/speech` requests when `openai/v1/audiospeech` is explicitly configured.
- Honor configured media capability paths and existing model mapping so deployments can select the intended synchronous DashScope media model, such as Qwen-Image, Wan image, or Qwen-TTS, without changing shared routing behavior.
- Convert DashScope successful media responses into gateway-compatible success responses rather than passing DashScope native JSON through unchanged.
- Preserve existing chat, embeddings, responses, rerank, conversations, Anthropic messages, async DashScope APIs, auth, domain, failover, retry, and model-mapping behavior.
- Keep media compatibility opt-in through explicit provider capabilities; do not infer image or speech support from provider type alone.
- Add focused plugin tests and documentation for the supported Qwen/DashScope media conversion behavior and limitations.

## Capabilities

### New Capabilities

- `qwen-dashscope-openai-media-conversion`: Qwen/DashScope ai-proxy conversion for explicitly configured OpenAI-compatible image generation and audio speech media requests.

### Modified Capabilities

- None.

## Impact

- Affected code: `extensions/ai-proxy/provider/qwen.go`, Qwen provider tests, ai-proxy integration tests, and ai-proxy documentation.
- Affected runtime behavior: Qwen/DashScope providers with explicit `openai/v1/imagegeneration` or `openai/v1/audiospeech` capability mappings can process OpenAI media requests through native DashScope endpoints.
- Affected APIs: OpenAI-style gateway paths `/v1/images/generations` and `/v1/audio/speech` for explicitly media-capable Qwen/DashScope provider configurations.
- GitNexus impact summary: Qwen-local methods targeted for extension are LOW risk (`TransformRequestBodyHeaders`, `TransformResponseBody`, `GetApiName`, `DefaultCapabilities`, existing chat/embedding helpers). Shared helpers are CRITICAL risk and should not be modified in this change: `ProviderConfig.handleRequestBody` (32 direct callers, 18 flows), `ProviderConfig.isSupportedAPI` (35 direct callers, 21 flows), and `getMappedModel` (65 impacted symbols, 25 flows).
- Dependencies: no new external service dependencies beyond existing DashScope upstream calls; strict OpenAI audio binary passthrough may require a separate follow-up if the first implementation returns the DashScope audio URL as a successful gateway payload.
