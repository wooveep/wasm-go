## 1. Safety and Baseline

- [x] 1.1 Run GitNexus impact analysis for each source symbol that will be edited, including request routing, request body handling, response body handling, streaming handling, and provider conversion helpers.
- [x] 1.2 Confirm the current `/v1/responses` native-path behavior and existing provider capability defaults with focused tests or test fixtures before changing implementation.
- [ ] 1.3 Add failing tests for native Responses routing, fallback routing, unsupported provider behavior, request conversion, response conversion, streaming conversion, and existing `/v1/messages` and `/v1/chat/completions` regressions.

## 2. Routing and Context Markers

- [ ] 2.1 Add or refine shared context keys for Responses fallback conversion without interfering with the existing Claude response conversion marker.
- [ ] 2.2 Update non-original protocol routing so `ApiNameResponses` keeps native routing when `openai/v1/responses` is supported by the active provider.
- [ ] 2.3 Update non-original protocol routing so `ApiNameResponses` falls back to `ApiNameChatCompletion` and `/v1/chat/completions` when only Chat Completions is supported.
- [ ] 2.4 Add explicit unsupported handling when the active provider supports neither Responses nor Chat Completions text generation.

## 3. Responses Request Conversion

- [ ] 3.1 Add Responses request model structs or parsing helpers for the supported fallback subset.
- [ ] 3.2 Convert string `input` into one Chat Completions user message.
- [ ] 3.3 Convert supported Responses message-array `input` items into Chat Completions `messages`.
- [ ] 3.4 Convert `instructions` into leading system or developer-level Chat Completions messages.
- [ ] 3.5 Map common generation fields, including `stream`, `temperature`, `top_p`, `presence_penalty`, `frequency_penalty`, and `max_output_tokens`.
- [ ] 3.6 Map safe Responses function tools and compatible `tool_choice` values to Chat Completions fields.
- [ ] 3.7 Reject or document unsupported fallback fields, including `previous_response_id`, `conversation`, hosted tools, and unmappable file or multimodal inputs.
- [ ] 3.8 Integrate request conversion before existing provider-specific Chat Completions request transformation so model mapping and provider conversion remain reused.

## 4. Responses Object Conversion

- [ ] 4.1 Add Responses response model structs or builders for fallback non-streaming responses.
- [ ] 4.2 Convert Chat Completions `choices[0].message.content` into Responses `output[].content[]` with `type: "output_text"`.
- [ ] 4.3 Convert Chat Completions `choices[0].message.refusal` into Responses refusal content.
- [ ] 4.4 Map Chat Completions usage tokens into Responses input, output, and total token usage.
- [ ] 4.5 Preserve response id, model, created timestamp, finish status, and error information where Responses has equivalent fields.
- [ ] 4.6 Integrate non-streaming conversion after provider response transformation and bypass it for native Responses providers.

## 5. Responses Streaming Conversion

- [ ] 5.1 Add a Chat Completions SSE to Responses SSE converter with per-request stream state stored in context.
- [ ] 5.2 Convert text deltas from `chat.completion.chunk` into `response.output_text.delta` events.
- [ ] 5.3 Convert finish chunks and `[DONE]` markers into Responses completion events such as `response.output_text.done` and `response.completed`.
- [ ] 5.4 Preserve usage, model, and finish reason in final stream state where available.
- [ ] 5.5 Integrate streaming conversion after provider streaming transformation and bypass it for native Responses SSE.

## 6. Documentation

- [ ] 6.1 Update `extensions/ai-proxy/README.md` to document `/v1/responses`, native capability priority, fallback behavior, supported fields, streaming behavior, and limitations.
- [ ] 6.2 Update `extensions/ai-proxy/README_EN.md` with the same Responses API behavior and limitations.
- [ ] 6.3 Add provider capability examples that include `openai/v1/responses`.

## 7. Verification

- [ ] 7.1 Run focused ai-proxy unit tests covering routing, request conversion, response conversion, streaming conversion, and regressions.
- [ ] 7.2 Run the broader relevant Go test suite for `extensions/ai-proxy`.
- [ ] 7.3 Run OpenSpec validation for `add-openai-responses-api-support`.
- [ ] 7.4 Run `graphify update .` after implementation code changes.
- [ ] 7.5 Run `gitnexus_detect_changes()` before committing and confirm affected symbols and flows match the expected ai-proxy scope.
