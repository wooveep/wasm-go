package main

import (
	"errors"

	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

func replaceMemoryRequestBody(ctx wrapper.HttpContext, input memoryMessageAssemblyInput, log log.Log) bool {
	original := ctx.GetByteSliceContext(memoryOriginalBodyContextKey, nil)
	if len(original) == 0 {
		log.Warnf("[ai-memory] original request body unavailable, fail open")
		return false
	}
	current, err := currentMemoryMessages(ctx)
	if err != nil {
		log.Warnf("[ai-memory] current request messages unavailable, fail open")
		return false
	}
	input.Current = current
	if !hasEffectiveMemoryInjection(input) {
		return false
	}
	replaced, err := sessionctx.ReplaceOpenAIChatMessages(original, assembleFinalMemoryMessages(input))
	if err != nil {
		log.Warnf("[ai-memory] request body assembly failed open: %v", err)
		return false
	}
	if err := memoryReplaceHTTPRequestBody(replaced); err != nil {
		log.Warnf("[ai-memory] request body replacement failed open: %v", err)
		return false
	}
	return true
}

func replaceMemoryRequestBodyWithRecentFallback(ctx wrapper.HttpContext, log log.Log) bool {
	recent := recentMemoryMessages(ctx)
	if len(recent) == 0 {
		return false
	}
	return replaceMemoryRequestBody(ctx, memoryMessageAssemblyInput{Recent: recent}, log)
}

func currentMemoryMessages(ctx wrapper.HttpContext) ([]sessionctx.OpenAIMessage, error) {
	messages, ok := ctx.GetContext(memoryCurrentMessagesContextKey).([]sessionctx.OpenAIMessage)
	if !ok {
		return nil, errors.New("current messages missing")
	}
	return append([]sessionctx.OpenAIMessage(nil), messages...), nil
}

func recentMemoryMessages(ctx wrapper.HttpContext) []sessionctx.OpenAIMessage {
	messages, ok := ctx.GetContext(memoryRecentMessagesContextKey).([]sessionctx.OpenAIMessage)
	if !ok {
		return nil
	}
	return append([]sessionctx.OpenAIMessage(nil), messages...)
}
