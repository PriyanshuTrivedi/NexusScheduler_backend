package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/chatbot/client"
	storemocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/chatbot/store"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestController_SendMessage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{
		MaxRetries: 0,
		BaseDelay:  0,
	})

	ctx := context.Background()
	userID := "user-123"
	conversationID := "conversation-123"

	now := time.Now()

	conversation := &entity.Conversation{
		ID:        conversationID,
		UserID:    userID,
		Status:    entity.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	userMessage := entity.Message{
		Role:    entity.RoleUser,
		Content: "Hello",
	}

	storedUserMessage := &entity.Message{
		ID:        "message-user-123",
		Role:      entity.RoleUser,
		Content:   "Hello",
		CreatedAt: now,
	}

	storedAssistantMessage := &entity.Message{
		ID:        "message-assistant-123",
		Role:      entity.RoleAssistant,
		Content:   "Hi there!",
		CreatedAt: now.Add(time.Second),
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, userID).
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, userID, conversationID).
		Return([]*entity.Message{}, nil)

	mockLLM.EXPECT().
		GenerateReply(ctx, []entity.Message{userMessage}).
		Return("Hi there!", nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, userMessage, "request-123").
		Return(storedUserMessage, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Hi there!",
		}, "").
		Return(storedAssistantMessage, nil)

	req := &entity.SendMessageRequest{
		UserID:               userID,
		Message:              "Hello",
		ClientIdempotencyKey: "request-123",
	}

	response, err := controller.SendMessage(ctx, req)

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.Equal(t, conversationID, response.Conversation.ID)
	assert.Equal(t, "Hi there!", response.Conversation.LastMessagePreview)
	assert.Equal(t, entity.RoleUser, response.UserMessage.Role)
	assert.Equal(t, entity.RoleAssistant, response.AssistantMessage.Role)
	assert.Equal(t, "Hi there!", response.AssistantMessage.Content)
	assert.False(t, response.IsNewConversation)
}

func TestController_SendMessage_CreateNewConversation(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	ctx := context.Background()
	userID := "user-123"
	conversationID := "new-conversation-123"
	now := time.Now()

	conversation := &entity.Conversation{
		ID:        conversationID,
		UserID:    userID,
		Status:    entity.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	userMessage := entity.Message{
		Role:    entity.RoleUser,
		Content: "Hello",
	}

	storedUserMessage := &entity.Message{
		ID:        "message-user-123",
		Role:      entity.RoleUser,
		Content:   "Hello",
		CreatedAt: now,
	}

	storedAssistantMessage := &entity.Message{
		ID:        "message-assistant-123",
		Role:      entity.RoleAssistant,
		Content:   "Hi!",
		CreatedAt: now.Add(time.Second),
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, userID).
		Return(nil, store.ErrConversationNotFound)

	mockStore.EXPECT().
		CreateConversation(ctx, userID).
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, userID, conversationID).
		Return([]*entity.Message{}, nil)

	mockLLM.EXPECT().
		GenerateReply(ctx, []entity.Message{userMessage}).
		Return("Hi!", nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, userMessage, "").
		Return(storedUserMessage, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Hi!",
		}, "").
		Return(storedAssistantMessage, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:  userID,
		Message: "Hello",
	})

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.True(t, response.IsNewConversation)
	assert.Equal(t, conversationID, response.Conversation.ID)
}

func TestController_SendMessage_GetActiveConversationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	mockStore.EXPECT().
		GetActiveConversation(gomock.Any(), "user-123").
		Return(nil, errors.New("database error"))

	response, err := controller.SendMessage(context.Background(), &entity.SendMessageRequest{
		UserID:  "user-123",
		Message: "Hello",
	})

	assert.Nil(t, response)
	assert.ErrorContains(t, err, "get active conversation")
}

func TestController_SendMessage_GenerateReplyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	now := time.Now()
	conversation := &entity.Conversation{
		ID:        "conversation-123",
		UserID:    "user-123",
		Status:    entity.StatusActive,
		UpdatedAt: now,
	}

	mockStore.EXPECT().
		GetActiveConversation(gomock.Any(), "user-123").
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(gomock.Any(), "user-123", "conversation-123").
		Return([]*entity.Message{}, nil)

	mockLLM.EXPECT().
		GenerateReply(gomock.Any(), gomock.Any()).
		Return("", errors.New("LLM failed"))

	response, err := controller.SendMessage(context.Background(), &entity.SendMessageRequest{
		UserID:  "user-123",
		Message: "Hello",
	})

	assert.Nil(t, response)
	assert.ErrorContains(t, err, "generate reply")
}

func TestController_SendMessage_IdleConversation(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	ctx := context.Background()
	userID := "user-123"
	conversationID := "new-conversation-123"
	now := time.Now()

	oldConversation := &entity.Conversation{
		ID:        "old-conversation-123",
		UserID:    userID,
		Status:    entity.StatusActive,
		UpdatedAt: now.Add(-6 * time.Minute),
	}

	newConversation := &entity.Conversation{
		ID:        conversationID,
		UserID:    userID,
		Status:    entity.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	userMessage := entity.Message{
		Role:    entity.RoleUser,
		Content: "Hello again",
	}

	storedUserMessage := &entity.Message{
		ID:      "message-user-123",
		Role:    entity.RoleUser,
		Content: "Hello again",
	}

	storedAssistantMessage := &entity.Message{
		ID:      "message-assistant-123",
		Role:    entity.RoleAssistant,
		Content: "Welcome back!",
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, userID).
		Return(oldConversation, nil)

	mockStore.EXPECT().
		CloseActiveConversation(ctx, userID, entity.CloseReasonIdleTimeout).
		Return(oldConversation, nil)

	mockStore.EXPECT().
		CreateConversation(ctx, userID).
		Return(newConversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, userID, conversationID).
		Return([]*entity.Message{}, nil)

	mockLLM.EXPECT().
		GenerateReply(ctx, []entity.Message{userMessage}).
		Return("Welcome back!", nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, userMessage, "").
		Return(storedUserMessage, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Welcome back!",
		}, "").
		Return(storedAssistantMessage, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:  userID,
		Message: "Hello again",
	})

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.True(t, response.IsNewConversation)
	assert.Equal(t, conversationID, response.Conversation.ID)
}

func TestController_CloseConversation_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	ctx := context.Background()
	now := time.Now()

	conversation := &entity.Conversation{
		ID:        "conversation-123",
		UserID:    "user-123",
		UpdatedAt: now,
	}

	closedConversation := &entity.Conversation{
		ID:        "conversation-123",
		UserID:    "user-123",
		Status:    entity.StatusClosed,
		UpdatedAt: now,
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, "user-123").
		Return(conversation, nil)

	mockStore.EXPECT().
		CloseActiveConversation(ctx, "user-123", entity.CloseReasonUser).
		Return(closedConversation, nil)

	response, err := controller.CloseConversation(ctx, &entity.CloseConversationRequest{
		UserID: "user-123",
	})

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.True(t, response.Closed)
	assert.Equal(t, "conversation-123", response.ConversationID)
	assert.Equal(t, now, response.ClosedAt)
}

func TestController_CloseConversation_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	mockStore.EXPECT().
		GetActiveConversation(gomock.Any(), "user-123").
		Return(nil, store.ErrConversationNotFound)

	response, err := controller.CloseConversation(context.Background(), &entity.CloseConversationRequest{
		UserID: "user-123",
	})

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.False(t, response.Closed)
}

func TestController_ListConversations_LimitHandling(t *testing.T) {
	tests := []struct {
		name           string
		requestedLimit int
		expectedLimit  int
	}{
		{
			name:           "default limit",
			requestedLimit: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "maximum limit",
			requestedLimit: 200,
			expectedLimit:  maxLimit,
		},
		{
			name:           "valid limit",
			requestedLimit: 10,
			expectedLimit:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := storemocks.NewMockStore(ctrl)
			mockLLM := clientmocks.NewMockLLM(ctrl)

			controller := New(mockStore, mockLLM, RetryConfig{})

			mockStore.EXPECT().
				ListConversations(gomock.Any(), "user-123", int32(tt.expectedLimit), int32(0)).
				Return([]*entity.Conversation{}, nil)

			response, err := controller.ListConversations(context.Background(), &entity.ListConversationsRequest{
				UserID: "user-123",
				Limit:  int32(tt.requestedLimit),
				Offset: int32(0),
			})

			assert.NoError(t, err)
			if !assert.NotNil(t, response) {
				return
			}

			assert.Empty(t, response.Conversations)
		})
	}
}

func TestController_GetMessages_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{})

	expectedMessages := []*entity.Message{
		{
			ID:      "message-123",
			Role:    entity.RoleUser,
			Content: "Hello",
		},
	}

	mockStore.EXPECT().
		GetMessages(gomock.Any(), "user-123", "conversation-123").
		Return(expectedMessages, nil)

	response, err := controller.GetMessages(context.Background(), &entity.GetMessagesRequest{
		UserID:         "user-123",
		ConversationID: "conversation-123",
	})

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.Equal(t, expectedMessages, response.Messages)
}

func TestController_SendMessage_RetryServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)

	controller := New(mockStore, mockLLM, RetryConfig{
		MaxRetries: 1,
		BaseDelay:  time.Nanosecond,
	})

	ctx := context.Background()
	userID := "user-123"
	conversationID := "conversation-123"
	now := time.Now()

	conversation := &entity.Conversation{
		ID:        conversationID,
		UserID:    userID,
		Status:    entity.StatusActive,
		UpdatedAt: now,
	}

	userMessage := entity.Message{
		Role:    entity.RoleUser,
		Content: "Hello",
	}

	storedUserMessage := &entity.Message{
		ID:        "message-user-123",
		Role:      entity.RoleUser,
		Content:   "Hello",
		CreatedAt: now,
	}

	storedAssistantMessage := &entity.Message{
		ID:        "message-assistant-123",
		Role:      entity.RoleAssistant,
		Content:   "Success",
		CreatedAt: now.Add(time.Second),
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, userID).
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, userID, conversationID).
		Return([]*entity.Message{}, nil)

	// First LLM call fails; retry succeeds.
	mockLLM.EXPECT().
		GenerateReply(ctx, []entity.Message{userMessage}).
		Return("", genai.APIError{Code: 503}).
		Times(1)

	mockLLM.EXPECT().
		GenerateReply(ctx, []entity.Message{userMessage}).
		Return("Success", nil).
		Times(1)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, userMessage, "request-123").
		Return(storedUserMessage, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Success",
		}, "").
		Return(storedAssistantMessage, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:               userID,
		Message:              "Hello",
		ClientIdempotencyKey: "request-123",
	})

	assert.NoError(t, err)
	if !assert.NotNil(t, response) {
		return
	}

	assert.Equal(t, "Success", response.AssistantMessage.Content)
	assert.Equal(t, conversationID, response.Conversation.ID)
}
