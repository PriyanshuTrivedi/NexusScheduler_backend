package client

import (
	"context"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
)

//go:generate mockgen -source=llm.go -destination=../../../gen/mocks/chatbot/client/llm_mock.go -package=mocks

// LLM defines the contract that every LLM provider must implement.
type LLM interface {
	GenerateReply(ctx context.Context, messages []entity.Message) (string, error)
}
