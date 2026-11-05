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
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestController_SendMessage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

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

	mockMCP.EXPECT().
		ListTools(ctx).
		Return([]entity.Tool{}, nil)

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		Return(&entity.LLMResponse{Text: "Hi there!"}, nil)

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

	require.NoError(t, err)
	require.NotNil(t, response)

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
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

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

	mockStore.EXPECT().
		GetActiveConversation(ctx, userID).
		Return(nil, store.ErrConversationNotFound)

	mockStore.EXPECT().
		CreateConversation(ctx, userID).
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, userID, conversationID).
		Return([]*entity.Message{}, nil)

	mockMCP.EXPECT().
		ListTools(ctx).
		Return([]entity.Tool{}, nil)

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		Return(&entity.LLMResponse{Text: "Hi!"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, userMessage, "").
		Return(&entity.Message{Role: entity.RoleUser, Content: "Hello"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Hi!",
		}, "").
		Return(&entity.Message{Role: entity.RoleAssistant, Content: "Hi!"}, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:  userID,
		Message: "Hello",
	})

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.True(t, response.IsNewConversation)
	assert.Equal(t, conversationID, response.Conversation.ID)
}

func TestController_SendMessage_GetActiveConversationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

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
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

	conversation := &entity.Conversation{
		ID:        "conversation-123",
		UserID:    "user-123",
		UpdatedAt: time.Now(),
	}

	mockStore.EXPECT().
		GetActiveConversation(gomock.Any(), "user-123").
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(gomock.Any(), "user-123", "conversation-123").
		Return([]*entity.Message{}, nil)

	mockMCP.EXPECT().
		ListTools(gomock.Any()).
		Return(nil, errors.New("MCP unavailable"))

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
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

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

	mockMCP.EXPECT().
		ListTools(ctx).
		Return([]entity.Tool{}, nil)

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		Return(&entity.LLMResponse{Text: "Welcome back!"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, userMessage, "").
		Return(&entity.Message{Role: entity.RoleUser, Content: "Hello again"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversationID, entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Welcome back!",
		}, "").
		Return(&entity.Message{Role: entity.RoleAssistant, Content: "Welcome back!"}, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:  userID,
		Message: "Hello again",
	})

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.True(t, response.IsNewConversation)
	assert.Equal(t, conversationID, response.Conversation.ID)
}

func TestController_SendMessage_ToolExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

	ctx := context.Background()
	now := time.Now()

	conversation := &entity.Conversation{
		ID:        "conversation-123",
		UserID:    "user-123",
		Status:    entity.StatusActive,
		UpdatedAt: now,
	}

	tool := entity.Tool{
		Name:        "get_booking",
		Description: "Get booking details",
	}

	call := entity.FunctionCall{
		ID:   "call-123",
		Name: "get_booking",
		Arguments: map[string]any{
			"booking_id": "booking-123",
		},
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, "user-123").
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, "user-123", conversation.ID).
		Return([]*entity.Message{}, nil)

	mockMCP.EXPECT().
		ListTools(ctx).
		Return([]entity.Tool{tool}, nil)

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		Return(&entity.LLMResponse{
			FunctionCalls: []entity.FunctionCall{call},
		}, nil)

	mockMCP.EXPECT().
		CallTool(ctx, "get_booking", call.Arguments).
		Return(map[string]any{
			"booking_id": "booking-123",
			"status":     "confirmed",
		}, nil)

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.LLMRequest) (*entity.LLMResponse, error) {
			require.Len(t, request.Messages, 3)

			assert.Equal(t, "model", request.Messages[1].Role)
			assert.Len(t, request.Messages[1].FunctionCalls, 1)

			assert.Equal(t, "user", request.Messages[2].Role)
			require.Len(t, request.Messages[2].FunctionResponses, 1)
			assert.Equal(t, "call-123", request.Messages[2].FunctionResponses[0].ID)
			assert.Equal(t, "get_booking", request.Messages[2].FunctionResponses[0].Name)

			return &entity.LLMResponse{
				Text: "Your booking is confirmed.",
			}, nil
		})

	mockStore.EXPECT().
		CreateMessage(ctx, conversation.ID, gomock.Any(), "").
		Return(&entity.Message{Role: entity.RoleUser, Content: "Check my booking"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversation.ID, gomock.Any(), "").
		Return(&entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Your booking is confirmed.",
		}, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:  "user-123",
		Message: "Check my booking",
	})

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "Your booking is confirmed.", response.AssistantMessage.Content)
}

func TestController_SendMessage_RetryServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{
		MaxRetries: 1,
		BaseDelay:  time.Nanosecond,
	})

	ctx := context.Background()
	now := time.Now()

	conversation := &entity.Conversation{
		ID:        "conversation-123",
		UserID:    "user-123",
		Status:    entity.StatusActive,
		UpdatedAt: now,
	}

	mockStore.EXPECT().
		GetActiveConversation(ctx, "user-123").
		Return(conversation, nil)

	mockStore.EXPECT().
		GetMessages(ctx, "user-123", conversation.ID).
		Return([]*entity.Message{}, nil)

	mockMCP.EXPECT().
		ListTools(ctx).
		Return([]entity.Tool{}, nil)

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		Return(nil, genai.APIError{Code: 503})

	mockLLM.EXPECT().
		GenerateResponse(ctx, gomock.Any()).
		Return(&entity.LLMResponse{Text: "Success"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversation.ID, gomock.Any(), "").
		Return(&entity.Message{Role: entity.RoleUser, Content: "Hello"}, nil)

	mockStore.EXPECT().
		CreateMessage(ctx, conversation.ID, gomock.Any(), "").
		Return(&entity.Message{Role: entity.RoleAssistant, Content: "Success"}, nil)

	response, err := controller.SendMessage(ctx, &entity.SendMessageRequest{
		UserID:  "user-123",
		Message: "Hello",
	})

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "Success", response.AssistantMessage.Content)
}

func TestController_CloseConversation_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

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

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.True(t, response.Closed)
	assert.Equal(t, "conversation-123", response.ConversationID)
	assert.Equal(t, now, response.ClosedAt)
}

func TestController_CloseConversation_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

	mockStore.EXPECT().
		GetActiveConversation(gomock.Any(), "user-123").
		Return(nil, store.ErrConversationNotFound)

	response, err := controller.CloseConversation(context.Background(), &entity.CloseConversationRequest{
		UserID: "user-123",
	})

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.False(t, response.Closed)
}

func TestController_ListConversations_LimitHandling(t *testing.T) {
	tests := []struct {
		name           string
		requestedLimit int32
		expectedLimit  int32
	}{
		{
			name:           "default limit",
			requestedLimit: 0,
			expectedLimit:  defaultConversationListLimit,
		},
		{
			name:           "maximum limit",
			requestedLimit: 200,
			expectedLimit:  maxConversationListLimit,
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
			mockMCP := clientmocks.NewMockMCPClient(ctrl)

			controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

			mockStore.EXPECT().
				ListConversations(gomock.Any(), "user-123", tt.expectedLimit, int32(0)).
				Return([]*entity.Conversation{}, nil)

			response, err := controller.ListConversations(context.Background(), &entity.ListConversationsRequest{
				UserID: "user-123",
				Limit:  tt.requestedLimit,
				Offset: 0,
			})

			require.NoError(t, err)
			require.NotNil(t, response)
			assert.Empty(t, response.Conversations)
		})
	}
}

func TestController_GetMessages_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)

	controller := New(mockStore, mockLLM, mockMCP, RetryConfig{})

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

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, expectedMessages, response.Messages)
}
