package client

import (
	"context"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
)

//go:generate mockgen -source=llm.go -destination=../../../gen/mocks/chatbot/client/llm_mock.go -package=mocks

// LLM defines the contract that every LLM provider must implement.
type LLM interface {
	GenerateResponse(ctx context.Context, request entity.LLMRequest) (*entity.LLMResponse, error)
	GenerateEmbeddings(ctx context.Context, chunks []string) ([]entity.Embedding, error)
}
