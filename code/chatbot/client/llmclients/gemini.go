package llmclients

import (
	"context"
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"google.golang.org/genai"
)

//go:generate mockgen -source=gemini.go -destination=../../../../gen/mocks/chatbot/client/llmclients/gemini_mock.go -package=mocks

type GeminiClient interface {
	GenerateResponse(ctx context.Context, request entity.LLMRequest) (*entity.LLMResponse, error)
	GenerateEmbeddings(ctx context.Context, chunks []string) ([]entity.Embedding, error)
}

type GeminiConfig struct {
	Model               string
	EmbeddingModel      string
	EmbeddingDimensions int32
}

type geminiClient struct {
	client       *genai.Client
	clientConfig GeminiConfig
}

func NewGeminiClient(ctx context.Context, apiKey string, cfg GeminiConfig) (GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create Gemini client: %w", err)
	}

	return &geminiClient{
		client:       client,
		clientConfig: cfg,
	}, nil
}

func (c *geminiClient) GenerateResponse(ctx context.Context, request entity.LLMRequest) (*entity.LLMResponse, error) {
	contents, err := convertMessages(request.Messages)
	if err != nil {
		return nil, fmt.Errorf("convert conversation messages: %w", err)
	}

	tools := convertTools(request.Tools)

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{
					Text: request.SystemInstruction,
				},
			},
		},
	}
	if len(tools) > 0 {
		config.Tools = tools
	}

	response, err := c.client.Models.GenerateContent(ctx, c.clientConfig.Model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("generate Gemini response: %w", err)
	}

	if response == nil {
		return nil, fmt.Errorf("Gemini returned an empty response")
	}

	result := &entity.LLMResponse{}

	if len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
		return nil, fmt.Errorf("Gemini returned no response content")
	}

	for _, part := range response.Candidates[0].Content.Parts {
		if part.Text != "" {
			result.Text += part.Text
		}

		if part.FunctionCall != nil {
			call := part.FunctionCall

			result.FunctionCalls = append(result.FunctionCalls, entity.FunctionCall{
				ID:               call.ID,
				Name:             call.Name,
				Arguments:        call.Args,
				ThoughtSignature: part.ThoughtSignature,
			})
		}
	}

	return result, nil
}

func (c *geminiClient) GenerateEmbeddings(ctx context.Context, chunks []string) ([]entity.Embedding, error) {
	contents, err := convertChunks(chunks)
	if err != nil {
		return nil, fmt.Errorf("convert chunks to Gemini content: %w", err)
	}

	response, err := c.client.Models.EmbedContent(
		ctx,
		c.clientConfig.EmbeddingModel,
		contents,
		&genai.EmbedContentConfig{
			TaskType:             "RETRIEVAL_DOCUMENT",
			OutputDimensionality: &c.clientConfig.EmbeddingDimensions,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("generate Gemini embeddings: %w", err)
	}

	if response == nil || len(response.Embeddings) != len(chunks) {
		return nil, fmt.Errorf("Gemini returned an unexpected number of embeddings")
	}

	embeddings := make([]entity.Embedding, 0, len(response.Embeddings))
	for i, embedding := range response.Embeddings {
		if embedding == nil || len(embedding.Values) == 0 {
			return nil, fmt.Errorf("Gemini returned an empty embedding for chunk %d", i)
		}

		embeddings = append(embeddings, entity.Embedding{
			Vector:  embedding.Values,
			Content: chunks[i],
		})
	}

	return embeddings, nil
}

func convertChunks(chunks []string) ([]*genai.Content, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("cannot generate embeddings for empty chunks")
	}

	contents := make([]*genai.Content, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == "" {
			return nil, fmt.Errorf("cannot generate embedding for an empty chunk")
		}

		contents = append(contents, &genai.Content{
			Parts: []*genai.Part{
				{Text: chunk},
			},
		})
	}

	return contents, nil
}

func convertMessages(messages []entity.LLMMessage) ([]*genai.Content, error) {
	contents := make([]*genai.Content, 0, len(messages))

	for _, message := range messages {
		role := message.Role
		if role == "assistant" {
			role = "model"
		}

		if role != "user" && role != "model" {
			return nil, fmt.Errorf("unsupported Gemini message role %q", message.Role)
		}

		content := &genai.Content{
			Role:  role,
			Parts: make([]*genai.Part, 0),
		}

		if message.Content != "" {
			content.Parts = append(content.Parts, &genai.Part{
				Text: message.Content,
			})
		}

		for _, call := range message.FunctionCalls {
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   call.ID,
					Name: call.Name,
					Args: call.Arguments,
				},
				ThoughtSignature: call.ThoughtSignature,
			})
		}

		for _, functionResponse := range message.FunctionResponses {
			content.Parts = append(content.Parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       functionResponse.ID,
					Name:     functionResponse.Name,
					Response: functionResponse.Response,
				},
			})
		}

		if len(content.Parts) == 0 {
			continue
		}

		contents = append(contents, content)
	}

	return contents, nil
}

func convertTools(tools []entity.Tool) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]*genai.FunctionDeclaration, 0, len(tools))

	for _, tool := range tools {
		declarations = append(declarations, &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: tool.InputSchema,
		})
	}

	return []*genai.Tool{
		{
			FunctionDeclarations: declarations,
		},
	}
}
