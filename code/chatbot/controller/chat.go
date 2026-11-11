package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/chunker"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/util"
	"google.golang.org/genai"
)

//go:generate mockgen -source=chat.go -destination=../../../gen/mocks/chatbot/controller/chat_mock.go -package=mocks

const (
	conversationIdleTimeout      = 5 * time.Minute
	defaultConversationListLimit = 20
	maxConversationListLimit     = 100
	maxLLMToolExecutionRounds    = 10
)

type Controller interface {
	SendMessage(ctx context.Context, req *entity.SendMessageRequest) (*entity.SendMessageResponse, error)
	CloseConversation(ctx context.Context, req *entity.CloseConversationRequest) (*entity.CloseConversationResponse, error)
	ListConversations(ctx context.Context, req *entity.ListConversationsRequest) (*entity.ListConversationsResponse, error)
	GetMessages(ctx context.Context, req *entity.GetMessagesRequest) (*entity.GetMessagesResponse, error)
	IndexDocuments(ctx context.Context, req *entity.IndexDocumentRequest) (*entity.IndexDocumentResponse, error)
	DeleteDocumentIndex(ctx context.Context, req *entity.DeleteDocumentIndexRequest) (*entity.DeleteDocumentIndexResponse, error)
}

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

type controller struct {
	store       store.Store
	llm         client.LLM
	mcp         client.MCPClient
	pinecone    client.PineconeClient
	chunker     chunker.Chunker
	retryConfig RetryConfig
}

func New(st store.Store, llm client.LLM, mcp client.MCPClient, pinecone client.PineconeClient, chunker chunker.Chunker, retryConfig RetryConfig) Controller {
	return &controller{
		store:       st,
		llm:         llm,
		mcp:         mcp,
		pinecone:    pinecone,
		chunker:     chunker,
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

	if conversation != nil && time.Since(conversation.UpdatedAt) >= conversationIdleTimeout {
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

	retrievedContext, err := c.retrieveContext(ctx, req.Message)
	if err != nil {
		return nil, fmt.Errorf("controller: retrieve document context: %w", err)
	}

	reply, err := c.generateReplyWithTools(ctx, history, retrievedContext)
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

func (c *controller) generateReplyWithTools(ctx context.Context, history []entity.Message, retrievedContext string) (string, error) {
	tools, err := c.mcp.ListTools(ctx)
	if err != nil {
		return "", fmt.Errorf("list MCP tools: %w", err)
	}

	messages := make([]entity.LLMMessage, 0, len(history)+maxLLMToolExecutionRounds*2)

	for _, message := range history {
		role := strings.ToLower(string(message.Role))
		if role == "assistant" {
			role = "model"
		}
		messages = append(messages, entity.LLMMessage{
			Role:    role,
			Content: message.Content,
		})
	}

	for toolRound := 0; ; toolRound++ {
		response, err := c.generateResponseWithRetry(ctx, entity.LLMRequest{
			Messages:          messages,
			Tools:             tools,
			SystemInstruction: util.BuildSystemInstruction(retrievedContext),
		})
		if err != nil {
			return "", fmt.Errorf("generate LLM response: %w", err)
		}

		if len(response.FunctionCalls) == 0 {
			return response.Text, nil
		}

		if toolRound >= maxLLMToolExecutionRounds {
			return "", fmt.Errorf("maximum tool-call toolRounds (%d) exceeded", maxLLMToolExecutionRounds)
		}

		// Preserve Gemini's function-call message in the conversation.
		messages = append(messages, entity.LLMMessage{
			Role:          "model",
			Content:       response.Text,
			FunctionCalls: response.FunctionCalls,
		})

		functionResponses := make([]entity.FunctionResponse, 0, len(response.FunctionCalls))

		for _, call := range response.FunctionCalls {
			result, err := c.mcp.CallTool(ctx, call.Name, call.Arguments)

			var toolResponse map[string]any

			if err != nil {
				toolResponse = map[string]any{
					"error": err.Error(),
				}
			} else {
				toolResponse, err = toResponseMap(result)
				if err != nil {
					toolResponse = map[string]any{
						"error": fmt.Sprintf("serialize tool result: %v", err),
					}
				}
			}

			functionResponses = append(functionResponses, entity.FunctionResponse{
				ID:       call.ID,
				Name:     call.Name,
				Response: toolResponse,
			})
		}

		// Gemini expects function responses in a user-role message.
		messages = append(messages, entity.LLMMessage{
			Role:              "user",
			FunctionResponses: functionResponses,
		})
	}
}

func (c *controller) generateResponseWithRetry(ctx context.Context, request entity.LLMRequest) (*entity.LLMResponse, error) {
	var err error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var response *entity.LLMResponse
		response, err = c.llm.GenerateResponse(ctx, request)
		if err == nil {
			return response, nil
		}

		var apiErr genai.APIError
		if !errors.As(err, &apiErr) || apiErr.Code < 500 || apiErr.Code > 599 {
			return nil, err
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
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, err
}

func toResponseMap(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}

	if result, ok := value.(map[string]any); ok {
		return result, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
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
	if time.Since(conversation.UpdatedAt) >= conversationIdleTimeout {
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
		limit = defaultConversationListLimit
	}
	if limit > maxConversationListLimit {
		limit = maxConversationListLimit
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

func (c *controller) IndexDocuments(ctx context.Context, req *entity.IndexDocumentRequest) (*entity.IndexDocumentResponse, error) {
	if err := util.ValidateIndexDocumentRequest(req); err != nil {
		return nil, fmt.Errorf("controller: validate index document request: %w", err)
	}

	text, err := util.ExtractPDFText(req.Content)
	if err != nil {
		return nil, fmt.Errorf("controller: extract PDF text: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("controller: extracted PDF text is empty")
	}

	chunks, err := c.chunker.Chunk(text)
	if err != nil {
		return nil, fmt.Errorf("controller: chunk document: %w", err)
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("controller: document produced no chunks")
	}

	embeddings, err := c.generateEmbeddingsWithRetry(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("controller: generate document embeddings: %w", err)
	}

	if len(embeddings) != len(chunks) {
		return nil, fmt.Errorf(
			"controller: embedding count mismatch: expected %d, got %d",
			len(chunks),
			len(embeddings),
		)
	}

	vectors := util.BuildPineconeVectors(
		req.ObjectID,
		req.OwnerUserID,
		req.IdempotencyKey,
		embeddings,
	)

	upserted, err := c.pinecone.UpsertVectors(ctx, vectors)
	if err != nil {
		return nil, fmt.Errorf("controller: upsert document vectors: %w", err)
	}

	return &entity.IndexDocumentResponse{
		Indexed:        true,
		ChunksUpserted: int32(upserted),
	}, nil
}

func (c *controller) generateEmbeddingsWithRetry(ctx context.Context, contents []string) ([]entity.Embedding, error) {
	var embeddings []entity.Embedding
	var err error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		embeddings, err = c.llm.GenerateEmbeddings(ctx, contents)
		if err == nil {
			return embeddings, nil
		}

		var apiErr genai.APIError
		if !errors.As(err, &apiErr) || apiErr.Code < 500 || apiErr.Code > 599 {
			return nil, err
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
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, err
}

func (c *controller) DeleteDocumentIndex(ctx context.Context, req *entity.DeleteDocumentIndexRequest) (*entity.DeleteDocumentIndexResponse, error) {
	if err := util.ValidateDeleteDocumentIndexRequest(req); err != nil {
		return nil, fmt.Errorf("controller: validate delete document index request: %w", err)
	}

	filter := util.BuildPineconeDocumentFilter(
		req.ObjectID,
		req.OwnerUserID,
	)

	if err := c.pinecone.DeleteVectorsByFilter(ctx, filter); err != nil {
		return nil, fmt.Errorf("controller: delete document vectors: %w", err)
	}

	return &entity.DeleteDocumentIndexResponse{
		Deleted: true,
	}, nil
}

func (c *controller) retrieveContext(ctx context.Context, query string) (string, error) {
	embeddings, err := c.llm.GenerateEmbeddings(ctx, []string{query})
	if err != nil {
		return "", fmt.Errorf("generate query embedding: %w", err)
	}
	if len(embeddings) != 1 {
		return "", fmt.Errorf("expected one query embedding, got %d", len(embeddings))
	}

	chunks, err := c.pinecone.SearchVectors(ctx, embeddings[0].Vector)
	if err != nil {
		return "", fmt.Errorf("search relevant document chunks: %w", err)
	}

	return strings.Join(chunks, "\n\n"), nil
}
