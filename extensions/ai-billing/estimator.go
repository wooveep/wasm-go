package main

import (
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

func tokenizerForModel(model string) (tokenizer.Codec, error) {
	return tokenizer.Get(tokenizerEncodingForModel(model))
}

func tokenizerEncodingForModel(model string) tokenizer.Encoding {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if isCl100kBaseModel(normalizedModel) {
		return tokenizer.Cl100kBase
	}
	return tokenizer.O200kBase
}

func isCl100kBaseModel(model string) bool {
	for _, family := range []string{
		"gpt-4",
		"gpt-3.5",
		"gpt-35",
		"ft:gpt-4",
		"ft:gpt-3.5",
		"ft:davinci-002",
		"ft:babbage-002",
		"text-embedding-ada-002",
		"davinci-002",
		"babbage-002",
	} {
		if matchesModelFamily(model, family) {
			return true
		}
	}
	return false
}

func matchesModelFamily(model, family string) bool {
	return model == family || strings.HasPrefix(model, family+"-") || strings.HasPrefix(model, family+":")
}
