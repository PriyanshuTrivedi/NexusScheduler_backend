package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	chunkermocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/chatbot/chunker"
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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{
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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

			controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

	controller := New(mockStore, mockLLM, mockMCP, nil, nil, RetryConfig{})

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

func makeControllerTestPDF(text string) []byte {
	var buffer bytes.Buffer
	offsets := []int{0}
	writeObject := func(id int, body string) {
		offsets = append(offsets, buffer.Len())
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	buffer.WriteString("%PDF-1.4\n")
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>")
	writeObject(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	stream := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
	writeObject(5, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	xrefOffset := buffer.Len()
	fmt.Fprintf(&buffer, "xref\n0 %d\n", len(offsets))
	buffer.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buffer, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(offsets), xrefOffset)
	return buffer.Bytes()
}

func TestController_IndexDocuments_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockLLM := clientmocks.NewMockLLM(ctrl)
	mockMCP := clientmocks.NewMockMCPClient(ctrl)
	mockPinecone := clientmocks.NewMockPineconeClient(ctrl)
	mockChunker := chunkermocks.NewMockChunker(ctrl)

	ctx := context.Background()
	pdfContent := makeControllerTestPDF("Nexus Scheduler document")
	chunks := []string{"chunk one", "chunk two"}
	embeddings := []entity.Embedding{
		{Vector: []float32{0.1, 0.2}, Content: chunks[0]},
		{Vector: []float32{0.3, 0.4}, Content: chunks[1]},
	}

	controller := New(mockStore, mockLLM, mockMCP, mockPinecone, mockChunker, RetryConfig{})
	mockChunker.EXPECT().
		Chunk(gomock.Any()).
		DoAndReturn(func(text string) ([]string, error) {
			assert.Contains(t, text, "Nexus Scheduler document")
			return chunks, nil
		})
	mockLLM.EXPECT().
		GenerateEmbeddings(ctx, chunks).
		Return(embeddings, nil)
	mockPinecone.EXPECT().
		UpsertVectors(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, vectors []entity.PineconeVector) (uint32, error) {
			require.Len(t, vectors, 2)
			assert.Equal(t, "object-1_0", vectors[0].ID)
			assert.Equal(t, "object-1", vectors[0].Metadata["object_id"])
			assert.Equal(t, "user-1", vectors[0].Metadata["owner_user_id"])
			assert.Equal(t, "idempotency-1", vectors[0].Metadata["idempotency_key"])
			return 2, nil
		})

	response, err := controller.IndexDocuments(ctx, &entity.IndexDocumentRequest{
		ObjectID: "object-1", OwnerUserID: "user-1", IdempotencyKey: "idempotency-1",
		DocumentType: entity.DocumentTypePDF, Content: pdfContent,
	})

	require.NoError(t, err)
	require.Equal(t, &entity.IndexDocumentResponse{Indexed: true, ChunksUpserted: 2}, response)
}

func TestController_IndexDocuments_ValidationError(t *testing.T) {
	controller := New(nil, nil, nil, nil, nil, RetryConfig{})
	response, err := controller.IndexDocuments(context.Background(), nil)
	require.Nil(t, response)
	require.ErrorContains(t, err, "validate index document request")
}

func TestController_IndexDocuments_ChunkingError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockChunker := chunkermocks.NewMockChunker(ctrl)
	controller := New(nil, nil, nil, nil, mockChunker, RetryConfig{})

	mockChunker.EXPECT().Chunk(gomock.Any()).Return(nil, errors.New("chunker failure"))
	response, err := controller.IndexDocuments(context.Background(), &entity.IndexDocumentRequest{
		ObjectID: "object-1", OwnerUserID: "user-1", IdempotencyKey: "key-1",
		DocumentType: entity.DocumentTypePDF, Content: makeControllerTestPDF("some text"),
	})
	require.Nil(t, response)
	require.ErrorContains(t, err, "chunk document")
}

func TestController_DeleteDocumentIndex_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockPinecone := clientmocks.NewMockPineconeClient(ctrl)
	controller := New(nil, nil, nil, mockPinecone, nil, RetryConfig{})

	mockPinecone.EXPECT().
		DeleteVectorsByFilter(context.Background(), entity.PineconeFilter{
			"object_id":     map[string]interface{}{"$eq": "object-1"},
			"owner_user_id": map[string]interface{}{"$eq": "user-1"},
		}).
		Return(nil)

	response, err := controller.DeleteDocumentIndex(context.Background(), &entity.DeleteDocumentIndexRequest{
		ObjectID: "object-1", OwnerUserID: "user-1",
	})
	require.NoError(t, err)
	require.Equal(t, &entity.DeleteDocumentIndexResponse{Deleted: true}, response)
}

func TestController_DeleteDocumentIndex_ValidationError(t *testing.T) {
	controller := New(nil, nil, nil, nil, nil, RetryConfig{})
	response, err := controller.DeleteDocumentIndex(context.Background(), nil)
	require.Nil(t, response)
	require.ErrorContains(t, err, "validate delete document index request")
}

func TestController_DeleteDocumentIndex_PineconeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockPinecone := clientmocks.NewMockPineconeClient(ctrl)
	controller := New(nil, nil, nil, mockPinecone, nil, RetryConfig{})

	mockPinecone.EXPECT().
		DeleteVectorsByFilter(gomock.Any(), gomock.Any()).
		Return(errors.New("Pinecone unavailable"))

	response, err := controller.DeleteDocumentIndex(context.Background(), &entity.DeleteDocumentIndexRequest{
		ObjectID: "object-1", OwnerUserID: "user-1",
	})
	require.Nil(t, response)
	require.ErrorContains(t, err, "delete document vectors")
}
