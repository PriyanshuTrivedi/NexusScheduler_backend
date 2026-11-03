package llmclients

import (
	"context"
	"errors"
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/util"
	"google.golang.org/genai"
)

//go:generate mockgen -source=gemini.go -destination=../../../../gen/mocks/chatbot/client/llmclients/gemini_mock.go -package=mocks

var (
	ErrGenerateContent = errors.New("generate content failed")
)

type geminiLLM struct {
	client *genai.Client
	model  string
}

// NewGeminiLLM creates a Gemini implementation using an initialized SDK client.
func NewGeminiLLM(geminiClient *genai.Client, model string) client.LLM {
	return &geminiLLM{
		client: geminiClient,
		model:  model,
	}
}

func (g *geminiLLM) GenerateReply(ctx context.Context, messages []entity.Message) (string, error) {
	contents, err := util.MessagesToGeminiContents(messages)
	if err != nil {
		return "", err
	}

	response, err := g.client.Models.GenerateContent(ctx, g.model, contents, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerateContent, err)
	}

	return util.GeminiResponseToText(response)
}
