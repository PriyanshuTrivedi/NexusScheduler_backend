package llmclients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func newGeminiTestClient(t *testing.T, handler http.Handler) *geminiClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  "mock-api-key",
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: server.URL + "/",
		},
	})
	require.NoError(t, err)

	return &geminiClient{
		client: client,
		clientConfig: GeminiConfig{
			Model:               "gemini-test-model",
			EmbeddingModel:      "gemini-test-embedding-model",
			EmbeddingDimensions: 768,
		},
	}
}

func writeGeminiResponse(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(body))
}

func TestNewGeminiClient(t *testing.T) {
	llm, err := NewGeminiClient(context.Background(), "mock-api-key", GeminiConfig{
		Model:               "gemini-test-model",
		EmbeddingModel:      "gemini-test-embedding-model",
		EmbeddingDimensions: 768,
	})

	require.NoError(t, err)
	require.NotNil(t, llm)

	gemini, ok := llm.(*geminiClient)
	require.True(t, ok)
	require.Equal(t, "gemini-test-model", gemini.clientConfig.Model)
	require.Equal(t, "gemini-test-embedding-model", gemini.clientConfig.EmbeddingModel)
	require.Equal(t, int32(768), gemini.clientConfig.EmbeddingDimensions)
}

func TestGeminiClient_GenerateResponse_Success(t *testing.T) {
	llm := newGeminiTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Contains(t, req.URL.Path, "gemini-test-model:generateContent")
		require.Equal(t, "mock-api-key", req.Header.Get("x-goog-api-key"))

		writeGeminiResponse(w, http.StatusOK, `{
			"candidates": [{
				"content": {
					"role": "model",
					"parts": [
						{"text": "Here are your booking details."},
						{
							"functionCall": {
								"id": "call-123",
								"name": "get_booking",
								"args": {"booking_id": "booking-123"}
							}
						}
					]
				},
				"finishReason": "STOP"
			}]
		}`)
	}))

	response, err := llm.GenerateResponse(context.Background(), entity.LLMRequest{
		Messages: []entity.LLMMessage{{Role: "user", Content: "Get my booking"}},
		Tools: []entity.Tool{{
			Name: "get_booking", Description: "Get booking details",
			InputSchema: map[string]any{"type": "object"},
		}},
	})

	require.NoError(t, err)
	require.Equal(t, "Here are your booking details.", response.Text)
	require.Len(t, response.FunctionCalls, 1)
	require.Equal(t, "call-123", response.FunctionCalls[0].ID)
	require.Equal(t, "get_booking", response.FunctionCalls[0].Name)
	require.Equal(t, "booking-123", response.FunctionCalls[0].Arguments["booking_id"])
}

func TestGeminiClient_GenerateResponse_Errors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "API error",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"code":400,"message":"Invalid request","status":"INVALID_ARGUMENT"}}`,
		},
		{name: "empty candidates", statusCode: http.StatusOK, body: `{"candidates":[]}`},
		{name: "candidate without content", statusCode: http.StatusOK, body: `{"candidates":[{}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := newGeminiTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeGeminiResponse(w, tt.statusCode, tt.body)
			}))
			response, err := llm.GenerateResponse(context.Background(), entity.LLMRequest{
				Messages: []entity.LLMMessage{{Role: "user", Content: "Hello"}},
			})
			require.Error(t, err)
			require.Nil(t, response)
		})
	}
}

func TestGeminiClient_GenerateResponse_ContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	llm := newGeminiTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("request should not reach the server for a canceled context")
	}))
	response, err := llm.GenerateResponse(ctx, entity.LLMRequest{
		Messages: []entity.LLMMessage{{Role: "user", Content: "Hello"}},
	})
	require.Error(t, err)
	require.Nil(t, response)
}

func TestGeminiClient_GenerateEmbeddings_Success(t *testing.T) {
	llm := newGeminiTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Contains(t, req.URL.Path, "gemini-test-embedding-model:batchEmbedContents")

		writeGeminiResponse(w, http.StatusOK, `{
			"embeddings": [
				{"values": [0.1, 0.2]},
				{"values": [0.3, 0.4]}
			]
		}`)
	}))

	embeddings, err := llm.GenerateEmbeddings(context.Background(), []string{"first chunk", "second chunk"})
	require.NoError(t, err)
	require.Len(t, embeddings, 2)
	require.Equal(t, []float32{0.1, 0.2}, embeddings[0].Vector)
	require.Equal(t, "first chunk", embeddings[0].Content)
	require.Equal(t, []float32{0.3, 0.4}, embeddings[1].Vector)
	require.Equal(t, "second chunk", embeddings[1].Content)
}

func TestGeminiClient_GenerateEmbeddings_Errors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		chunks     []string
	}{
		{
			name:       "API error",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"code":400,"message":"Invalid request","status":"INVALID_ARGUMENT"}}`,
			chunks:     []string{"first", "second"},
		},
		{
			name:       "unexpected embedding count",
			statusCode: http.StatusOK,
			body:       `{"embeddings":[{"values":[0.1]}]}`,
			chunks:     []string{"first", "second"},
		},
		{
			name:       "empty embedding vector",
			statusCode: http.StatusOK,
			body:       `{"embeddings":[{"values":[]}]}`,
			chunks:     []string{"first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := newGeminiTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeGeminiResponse(w, tt.statusCode, tt.body)
			}))
			embeddings, err := llm.GenerateEmbeddings(context.Background(), tt.chunks)
			require.Error(t, err)
			require.Nil(t, embeddings)
		})
	}
}

func TestGeminiClient_GenerateEmbeddings_ContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	llm := newGeminiTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("request should not reach the server for a canceled context")
	}))
	embeddings, err := llm.GenerateEmbeddings(ctx, []string{"chunk"})
	require.Error(t, err)
	require.Nil(t, embeddings)
}

func TestConvertChunks(t *testing.T) {
	contents, err := convertChunks([]string{"first", "second"})
	require.NoError(t, err)
	require.Len(t, contents, 2)
	require.Equal(t, "first", contents[0].Parts[0].Text)
	require.Equal(t, "second", contents[1].Parts[0].Text)

	_, err = convertChunks(nil)
	require.Error(t, err)
	_, err = convertChunks([]string{"valid", ""})
	require.Error(t, err)
}

func TestConvertMessages(t *testing.T) {
	contents, err := convertMessages([]entity.LLMMessage{
		{Role: "user", Content: "Get my booking"},
		{
			Role: "assistant",
			FunctionCalls: []entity.FunctionCall{{
				ID: "call-123", Name: "get_booking",
				Arguments:        map[string]any{"booking_id": "booking-123"},
				ThoughtSignature: []byte("thought-signature"),
			}},
		},
		{
			Role: "user",
			FunctionResponses: []entity.FunctionResponse{{
				ID: "call-123", Name: "get_booking",
				Response: map[string]any{"status": "CONFIRMED"},
			}},
		},
	})

	require.NoError(t, err)
	require.Len(t, contents, 3)
	require.Equal(t, "user", contents[0].Role)
	require.Equal(t, "model", contents[1].Role)
	require.NotNil(t, contents[1].Parts[0].FunctionCall)
	require.Equal(t, []byte("thought-signature"), contents[1].Parts[0].ThoughtSignature)
	require.NotNil(t, contents[2].Parts[0].FunctionResponse)
}

func TestConvertMessages_UnsupportedRole(t *testing.T) {
	contents, err := convertMessages([]entity.LLMMessage{{Role: "system", Content: "System prompt"}})
	require.Nil(t, contents)
	require.Error(t, err)
}

func TestConvertMessages_SkipsEmptyMessage(t *testing.T) {
	contents, err := convertMessages([]entity.LLMMessage{
		{Role: "user"},
		{Role: "assistant", Content: "response"},
	})
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "model", contents[0].Role)
}

func TestConvertTools(t *testing.T) {
	tools := convertTools([]entity.Tool{{
		Name: "get_booking", Description: "Get booking details",
		InputSchema: map[string]any{"type": "object"},
	}})

	require.Len(t, tools, 1)
	require.Len(t, tools[0].FunctionDeclarations, 1)
	require.Equal(t, "get_booking", tools[0].FunctionDeclarations[0].Name)
	require.Nil(t, convertTools(nil))
}
