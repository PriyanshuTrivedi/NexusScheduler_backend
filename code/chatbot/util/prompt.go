package util

import (
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/prompt"
)

func BuildSystemInstruction(retrievedContext string) string {
	if retrievedContext == "" {
		return prompt.SystemInstruction
	}

	retrievalPrompt := strings.Replace(
		prompt.RetrievalInstruction,
		"{{.RetrievedContext}}",
		retrievedContext,
		1,
	)

	return prompt.SystemInstruction + "\n\n" + retrievalPrompt
}
