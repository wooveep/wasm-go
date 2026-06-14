## ADDED Requirements

### Requirement: Explicit Qwen DashScope media capability gate
The ai-proxy Qwen provider SHALL process OpenAI-compatible image generation and audio speech media APIs only when the provider configuration explicitly enables the corresponding capability mapping.

#### Scenario: Explicit image capability enables native image generation conversion
- **WHEN** a Qwen provider configuration contains `capabilities.openai/v1/imagegeneration`
- **AND** `qwenEnableCompatible` is not enabled
- **AND** a client sends `POST /v1/images/generations`
- **THEN** ai-proxy SHALL treat the request as `ApiNameImageGeneration`
- **AND** ai-proxy SHALL route the upstream request to the configured DashScope image generation path
- **AND** ai-proxy SHALL convert the request body to DashScope native multimodal-generation JSON.

#### Scenario: Explicit audio capability enables native speech conversion
- **WHEN** a Qwen provider configuration contains `capabilities.openai/v1/audiospeech`
- **AND** `qwenEnableCompatible` is not enabled
- **AND** a client sends `POST /v1/audio/speech`
- **THEN** ai-proxy SHALL treat the request as `ApiNameAudioSpeech`
- **AND** ai-proxy SHALL route the upstream request to the configured DashScope speech generation path
- **AND** ai-proxy SHALL convert the request body to DashScope native Qwen-TTS JSON.

#### Scenario: Missing media capability stays unsupported
- **WHEN** a Qwen provider configuration does not contain `capabilities.openai/v1/imagegeneration` or `capabilities.openai/v1/audiospeech`
- **THEN** ai-proxy SHALL NOT synthesize those capabilities from provider type alone
- **AND** media requests for the missing capability SHALL continue to be rejected as unsupported.

#### Scenario: Qwen default capabilities do not include media
- **WHEN** a Qwen provider is created without explicit media capabilities
- **THEN** its default capabilities SHALL NOT include `openai/v1/imagegeneration`
- **AND** its default capabilities SHALL NOT include `openai/v1/audiospeech`.

### Requirement: Qwen DashScope image generation request conversion
The ai-proxy Qwen provider SHALL convert supported OpenAI image generation request fields into DashScope multimodal-generation request JSON.

#### Scenario: Prompt is converted to DashScope image message
- **WHEN** a client sends an image generation request with `model` and `prompt`
- **THEN** ai-proxy SHALL apply existing model mapping to `model`
- **AND** the DashScope request body SHALL contain `input.messages[0].role` equal to `user`
- **AND** the DashScope request body SHALL contain `input.messages[0].content[0].text` equal to the OpenAI `prompt`.

#### Scenario: Image generation parameters are mapped safely
- **WHEN** a client sends an image generation request with supported parameters such as `n`, `size`, or `seed`
- **THEN** ai-proxy SHALL map those values into DashScope `parameters`
- **AND** ai-proxy SHALL normalize OpenAI `size` values from `WIDTHxHEIGHT` to DashScope `WIDTH*HEIGHT`.

#### Scenario: Existing model mapping selects the DashScope image model
- **WHEN** a client sends an image generation request with an OpenAI-style `model`
- **AND** the provider has a model mapping for that model
- **THEN** ai-proxy SHALL use the existing provider model mapping result as the DashScope `model`
- **AND** ai-proxy SHALL NOT infer a default image model from the Qwen provider type.

#### Scenario: Unsupported image fields do not corrupt DashScope request
- **WHEN** a client sends OpenAI image generation fields that do not have a safe DashScope mapping
- **THEN** ai-proxy SHALL ignore those fields or return a clear transform error
- **AND** ai-proxy SHALL NOT forward unsupported fields in a way that changes the DashScope native request contract.

### Requirement: Qwen DashScope image generation response conversion
The ai-proxy Qwen provider SHALL convert successful DashScope image generation JSON responses into OpenAI-style image generation JSON responses.

#### Scenario: DashScope image URLs become OpenAI image data
- **WHEN** DashScope returns a successful image generation response containing image URLs under `output.choices[].message.content[]`
- **THEN** ai-proxy SHALL return JSON containing `created`
- **AND** ai-proxy SHALL return `data[]` entries whose `url` values are the DashScope image URLs.

#### Scenario: DashScope image usage is preserved when available
- **WHEN** DashScope returns image generation usage metadata
- **THEN** ai-proxy SHALL preserve the supported usage fields in the converted response where the ai-proxy OpenAI-style image response model supports them.

#### Scenario: DashScope image errors remain diagnostic
- **WHEN** DashScope returns an image generation error response
- **THEN** ai-proxy SHALL NOT convert it into a false success response
- **AND** the client SHALL receive a clear error or unchanged provider error body.

### Requirement: Qwen DashScope audio speech request conversion
The ai-proxy Qwen provider SHALL convert supported OpenAI audio speech request fields into DashScope Qwen-TTS multimodal-generation request JSON.

#### Scenario: Audio input and voice are converted
- **WHEN** a client sends an audio speech request with `model`, `input`, and `voice`
- **THEN** ai-proxy SHALL apply existing model mapping to `model`
- **AND** the DashScope request body SHALL contain `input.text` equal to the OpenAI `input`
- **AND** the DashScope request body SHALL contain `input.voice` equal to the OpenAI `voice`.

#### Scenario: DashScope TTS extension fields are preserved when present
- **WHEN** a client sends supported DashScope TTS extension fields such as `language_type`, `instructions`, or `optimize_instructions`
- **THEN** ai-proxy SHALL map those fields into the DashScope `input` object.

#### Scenario: Existing model mapping selects the DashScope TTS model
- **WHEN** a client sends an audio speech request with an OpenAI-style `model`
- **AND** the provider has a model mapping for that model
- **THEN** ai-proxy SHALL use the existing provider model mapping result as the DashScope `model`
- **AND** ai-proxy SHALL NOT infer a default TTS model from the Qwen provider type.

#### Scenario: Unsupported audio fields do not corrupt DashScope request
- **WHEN** a client sends OpenAI audio speech fields that do not have a safe DashScope mapping
- **THEN** ai-proxy SHALL ignore those fields or return a clear transform error
- **AND** ai-proxy SHALL NOT forward unsupported fields in a way that changes the DashScope native request contract.

### Requirement: Qwen DashScope audio speech response conversion
The ai-proxy Qwen provider SHALL convert successful DashScope Qwen-TTS JSON responses into a documented gateway success JSON response containing audio result metadata.

#### Scenario: DashScope audio URL becomes gateway success data
- **WHEN** DashScope returns a successful Qwen-TTS response containing `output.audio.url`
- **THEN** ai-proxy SHALL return a JSON success response containing the audio URL
- **AND** the response SHALL include audio `id` and `expires_at` when DashScope returns them.

#### Scenario: DashScope audio usage is preserved when available
- **WHEN** DashScope returns Qwen-TTS usage metadata
- **THEN** ai-proxy SHALL preserve supported token or character usage fields in the converted response.

#### Scenario: DashScope audio errors remain diagnostic
- **WHEN** DashScope returns a Qwen-TTS error response
- **THEN** ai-proxy SHALL NOT convert it into a false success response
- **AND** the client SHALL receive a clear error or unchanged provider error body.

### Requirement: Existing Qwen provider behavior remains compatible
The ai-proxy Qwen provider SHALL preserve existing non-media behavior while adding media conversion.

#### Scenario: Existing Qwen APIs are unchanged
- **WHEN** clients use Qwen chat completions, embeddings, rerank, responses, conversations, Anthropic messages, async AIGC, or async task APIs
- **THEN** ai-proxy SHALL preserve the existing request and response behavior for those APIs.

#### Scenario: Compatible mode remains pass-through
- **WHEN** `qwenEnableCompatible` is enabled for a Qwen provider
- **THEN** ai-proxy SHALL preserve existing compatible-mode pass-through behavior
- **AND** media conversion SHALL NOT change compatible-mode chat, embeddings, responses, files, batches, rerank, conversations, or Anthropic messages behavior.

#### Scenario: Compatible mode media capability remains pass-through
- **WHEN** `qwenEnableCompatible` is enabled for a Qwen provider
- **AND** the provider explicitly configures `openai/v1/imagegeneration` or `openai/v1/audiospeech`
- **THEN** ai-proxy SHALL route media requests to the configured capability path without rewriting them into DashScope native media JSON
- **AND** existing compatible-mode model mapping behavior SHALL remain unchanged.
