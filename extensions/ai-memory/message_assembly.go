package main

import (
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
)

type memoryMessageAssemblyInput struct {
	Current       []sessionctx.OpenAIMessage
	MemoryMessage *sessionctx.OpenAIMessage
	Recent        []sessionctx.OpenAIMessage
}

func assembleFinalMemoryMessages(input memoryMessageAssemblyInput) []sessionctx.OpenAIMessage {
	highPriority := make([]sessionctx.OpenAIMessage, 0)
	remaining := make([]sessionctx.OpenAIMessage, 0, len(input.Current))
	currentUserMessages := 0
	for _, message := range input.Current {
		if message.Role == "user" {
			currentUserMessages++
		}
		if isHighPriorityMemoryMessage(message) {
			highPriority = append(highPriority, message)
			continue
		}
		remaining = append(remaining, message)
	}

	finalMessages := make([]sessionctx.OpenAIMessage, 0, len(highPriority)+1+len(input.Recent)+len(remaining))
	finalMessages = append(finalMessages, highPriority...)
	if input.MemoryMessage != nil {
		finalMessages = append(finalMessages, *input.MemoryMessage)
	}
	if currentUserMessages <= 1 {
		finalMessages = append(finalMessages, input.Recent...)
	}
	finalMessages = append(finalMessages, remaining...)
	return finalMessages
}

func isHighPriorityMemoryMessage(message sessionctx.OpenAIMessage) bool {
	return message.Role == "system" || message.Role == "developer"
}

func memoryAssemblyInputFromResponse(response memoryConsoleAssembleResponse) memoryMessageAssemblyInput {
	switch response.Decision {
	case memoryAssembleDecisionInject:
		input := memoryMessageAssemblyInput{
			Recent: memoryAssembleMessagesToOpenAI(response.RecentMessages),
		}
		if response.MemoryMessage != nil {
			message := response.MemoryMessage.toOpenAIMessage()
			input.MemoryMessage = &message
		}
		return input
	case memoryAssembleDecisionRecentOnly:
		return memoryMessageAssemblyInput{
			Recent: memoryAssembleMessagesToOpenAI(response.RecentMessages),
		}
	default:
		return memoryMessageAssemblyInput{}
	}
}
