package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/util"
	"google.golang.org/genai"
)

//go:generate mockgen -source=chat.go -destination=../../../gen/mocks/chatbot/controller/chat_mock.go -package=mocks

const (
	idleTimeout  = 5 * time.Minute
	defaultLimit = 20
	maxLimit     = 100
)

type Controller interface {
	SendMessage(ctx context.Context, req *entity.SendMessageRequest) (*entity.SendMessageResponse, error)
	CloseConversation(ctx context.Context, req *entity.CloseConversationRequest) (*entity.CloseConversationResponse, error)
	ListConversations(ctx context.Context, req *entity.ListConversationsRequest) (*entity.ListConversationsResponse, error)
	GetMessages(ctx context.Context, req *entity.GetMessagesRequest) (*entity.GetMessagesResponse, error)
}

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

type controller struct {
	store       store.Store
	llm         client.LLM
	retryConfig RetryConfig
}

func New(st store.Store, llm client.LLM, retryConfig RetryConfig) Controller {
	return &controller{
		store:       st,
		llm:         llm,
		retryConfig: retryConfig,
	}
}

func (c *controller) SendMessage(ctx context.Context, req *entity.SendMessageRequest) (*entity.SendMessageResponse, error) {
	if err := util.ValidateSendMessageRequest(req); err != nil {
		return nil, err
	}

	conversation, err := c.store.GetActiveConversation(ctx, req.UserID)
	if err != nil && !errors.Is(err, store.ErrConversationNotFound) {
		return nil, fmt.Errorf("controller: get active conversation: %w", err)
	}

	isNewConversation := false

	if conversation != nil && time.Since(conversation.UpdatedAt) >= idleTimeout {
		_, err = c.store.CloseActiveConversation(ctx, req.UserID, entity.CloseReasonIdleTimeout)
		if err != nil {
			return nil, fmt.Errorf("controller: close idle conversation: %w", err)
		}
		conversation = nil
	}

	if conversation == nil {
		conversation, err = c.store.CreateConversation(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("controller: create conversation: %w", err)
		}
		isNewConversation = true
	}

	storedMessages, err := c.store.GetMessages(ctx, req.UserID, conversation.ID)
	if err != nil {
		return nil, fmt.Errorf("controller: get conversation messages: %w", err)
	}

	history := make([]entity.Message, 0, len(storedMessages)+1)
	for _, msg := range storedMessages {
		history = append(history, *msg)
	}

	userMessage := entity.Message{
		Role:    entity.RoleUser,
		Content: req.Message,
	}
	history = append(history, userMessage)

	reply, err := c.generateReplyWithRetry(ctx, history)
	if err != nil {
		return nil, fmt.Errorf("controller: generate reply: %w", err)
	}

	savedUserMessage, err := c.store.CreateMessage(
		ctx,
		conversation.ID,
		userMessage,
		req.ClientIdempotencyKey,
	)
	if err != nil {
		return nil, fmt.Errorf("controller: save user message: %w", err)
	}

	assistantMessage := entity.Message{
		Role:    entity.RoleAssistant,
		Content: reply,
	}

	savedAssistantMessage, err := c.store.CreateMessage(
		ctx,
		conversation.ID,
		assistantMessage,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("controller: save assistant message: %w", err)
	}

	conversation.LastMessagePreview = reply
	conversation.UpdatedAt = savedAssistantMessage.CreatedAt

	return &entity.SendMessageResponse{
		Conversation:      conversation,
		UserMessage:       savedUserMessage,
		AssistantMessage:  savedAssistantMessage,
		IsNewConversation: isNewConversation,
	}, nil
}

func (c *controller) generateReplyWithRetry(ctx context.Context, history []entity.Message) (string, error) {
	var err error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		var reply string
		reply, err = c.llm.GenerateReply(ctx, history)
		if err == nil {
			return reply, nil
		}

		var apiErr genai.APIError
		if !errors.As(err, &apiErr) || apiErr.Code < 500 || apiErr.Code > 599 {
			return "", err
		}

		if attempt == c.retryConfig.MaxRetries {
			break
		}

		maxDelay := c.retryConfig.BaseDelay * time.Duration(1<<attempt)
		delay := time.Duration(rand.Int63n(int64(maxDelay) + 1))

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}

	return "", err
}

func (c *controller) CloseConversation(ctx context.Context, req *entity.CloseConversationRequest) (*entity.CloseConversationResponse, error) {
	if err := util.ValidateCloseConversationRequest(req); err != nil {
		return nil, err
	}

	conversation, err := c.store.GetActiveConversation(ctx, req.UserID)
	if conversation == nil || errors.Is(err, store.ErrConversationNotFound) {
		return &entity.CloseConversationResponse{Closed: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("controller: get active conversation: %w", err)
	}

	reason := entity.CloseReasonUser
	if time.Since(conversation.UpdatedAt) >= idleTimeout {
		reason = entity.CloseReasonIdleTimeout
	}

	conversation, err = c.store.CloseActiveConversation(
		ctx,
		req.UserID,
		reason,
	)
	if errors.Is(err, store.ErrConversationNotFound) {
		return &entity.CloseConversationResponse{Closed: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("controller: close conversation: %w", err)
	}

	return &entity.CloseConversationResponse{
		ConversationID: conversation.ID,
		Closed:         true,
		ClosedAt:       conversation.UpdatedAt,
	}, nil
}

func (c *controller) ListConversations(ctx context.Context, req *entity.ListConversationsRequest) (*entity.ListConversationsResponse, error) {
	if err := util.ValidateListConversationsRequest(req); err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	conversations, err := c.store.ListConversations(
		ctx,
		req.UserID,
		limit,
		req.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("controller: list conversations: %w", err)
	}

	return &entity.ListConversationsResponse{
		Conversations: conversations,
	}, nil
}

func (c *controller) GetMessages(ctx context.Context, req *entity.GetMessagesRequest) (*entity.GetMessagesResponse, error) {
	if err := util.ValidateGetMessagesRequest(req); err != nil {
		return nil, err
	}

	messages, err := c.store.GetMessages(ctx, req.UserID, req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("controller: get messages: %w", err)
	}

	return &entity.GetMessagesResponse{
		Messages: messages,
	}, nil
}
