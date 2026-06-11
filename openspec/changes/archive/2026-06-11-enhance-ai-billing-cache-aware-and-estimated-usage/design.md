## Context

`extensions/ai-billing` builds request-level billing events after AI responses and currently records usage through provider response parsing. GitNexus shows the relevant flow as `onHttpResponseBody` and `onHttpStreamingResponseBody` calling `recordUsage`, which in turn uses `pkg/tokenusage.GetTokenUsage` and cache-aware split helpers before event delivery. This change expands that provider-first flow and adds a conservative text tokenizer fallback for cases where provider usage is unavailable.

The key constraints are that provider usage remains authoritative, local tokenization only provides basic usage estimates, and streaming interruption must still produce at most one billing event when enough request lifecycle information is available.

## Goals / Non-Goals

**Goals:**

- Preserve provider usage priority for every provider response that includes usable usage data.
- Parse the first batch of cache-aware usage shapes across OpenAI-compatible/GLM, Kimi, DeepSeek, Qwen, Claude/Anthropic, and Gemini payloads.
- Persist the complete raw provider usage object under `usage.details.provider_usage` whenever provider usage is used.
- Estimate basic input, output, and total token usage with `github.com/tiktoken-go/tokenizer` when provider usage is absent.
- Use `o200k_base` as the required fallback vocabulary for unknown, empty, or unmapped models.
- Deliver one billing event for normal streaming completion and also for stream termination caused by client interruption when a prior event has not already been delivered.
- Prevent duplicate billing events through a request-scoped delivered marker.

**Non-Goals:**

- Estimate cache hit, cache miss, cache creation, audio, image, tool-call-internal, or multimodal token counts locally.
- Replace provider usage with tokenizer estimates when provider usage exists.
- Guarantee exact parity between tokenizer estimates and provider-specific billing counts.
- Emit cache-aware fields or `details.provider_usage` for estimated usage events.

## Decisions

1. Provider usage is normalized before estimation fallback.

   The usage path SHALL first attempt to parse provider usage into a normalized usage result with `usage_source=provider`. Only when no provider usage is present or usable SHALL the local estimator run. This keeps provider-specific billing counts authoritative and avoids estimate drift overriding provider truth.

   Alternative considered: always estimate and compare with provider usage. This was rejected because it adds cost and ambiguity without changing the billing source of truth.

2. Provider raw usage is retained as event details only for provider-sourced events.

   The parser SHALL retain the complete provider usage object, including fields that are not interpreted in the first batch such as Qwen cache creation subfields. Cache hit and miss fields SHALL be populated only when the provider payload contains supported cache-aware fields and the derived values pass non-negative clamping rules.

   Alternative considered: retain only normalized fields. This was rejected because billing-service consumers need the original payload for auditability and future provider-specific interpretation.

3. Estimation uses structured text extraction, not whole request JSON.

   The estimator SHALL extract text from Chat Completions `messages[].content`, Responses `input` and `instructions`, and Completions `prompt`. If text cannot be extracted structurally, the estimator SHALL fail closed to `usage_source=missing` rather than tokenizing the complete JSON body and severely overcounting metadata.

   Alternative considered: tokenize the raw JSON request body. This was rejected because it would produce misleading input counts and could charge for protocol scaffolding instead of prompt text.

4. Streaming output estimation counts only text already sent to the client.

   The streaming response path SHALL accumulate text deltas as they are forwarded. If final provider usage never arrives, the fallback estimate SHALL use only accumulated deltas, so a client interruption does not charge for text that was never delivered downstream.

   Alternative considered: count the upstream response buffer even after client interruption. This was rejected because the requirement is to estimate delivered output, not generated-but-unsent output.

5. Stream lifecycle delivery uses an idempotent request marker.

   The response-body end-of-stream path SHALL continue to deliver the normal event. A stream-done hook SHALL check a `ctxBillingDelivered`-style marker and deliver a fallback event only if the normal path did not already deliver. The marker SHALL be set before dispatching delivery to prevent duplicate attempts from reentrant lifecycle callbacks.

   Alternative considered: infer duplicate delivery only from the event idempotency key at the billing service. This was rejected because the plugin must avoid duplicate callouts where it can, while still preserving event idempotency for retries.

## Risks / Trade-offs

- Tokenizer vocabulary adds binary size, expected to be about 4 MB for OpenAI vocabularies -> Accept the size increase because estimation is required only when provider usage is absent.
- Local token estimates can differ from provider billing counts -> Emit `usage_source=estimated`, keep estimates basic, and never override provider usage.
- Provider payload shapes vary and can include invalid cache values -> Apply non-negative validation and clamp cached tokens to input tokens before deriving hit and miss counts.
- Stream-done fallback may run with partial response data -> Count only emitted deltas and mark the event as estimated or missing according to available extraction results.
- Retaining raw provider usage can increase event payload size -> Retain only the provider usage object, not full request or response bodies.

## Migration Plan

1. Add tokenizer dependency and model-to-vocabulary mapping with fallback to `o200k_base`.
2. Extend provider usage parsing and event construction while preserving existing provider-sourced behavior for basic usage.
3. Add structured request and response text extraction for estimation.
4. Add streaming delta accumulation and delivered-marker guarded stream-done fallback.
5. Add unit tests for provider cache-aware shapes, estimation outputs, missing fallback, and duplicate-prevention behavior.
6. Run existing `ai-billing` and `pkg/tokenusage` tests, then update Graphify after implementation changes.

Rollback is to disable the estimator and stream-done fallback paths while keeping provider usage parsing changes isolated behind the normalized usage source function.

## Open Questions

- None for the proposal. Implementation should verify exact `tiktoken-go/tokenizer` API names against the selected module version before coding the estimator.
