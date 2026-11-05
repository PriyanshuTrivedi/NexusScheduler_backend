package llmclients

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockGeminiClient(transport http.RoundTripper) *genai.Client {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  "mock-api-key",
		Backend: genai.BackendGeminiAPI,
		HTTPClient: &http.Client{
			Transport: transport,
		},
		HTTPOptions: genai.HTTPOptions{
			BaseURL: "https://mock-gemini.example.com/",
		},
	})
	if err != nil {
		panic(err)
	}

	return client
}

func mockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestNewGeminiClient(t *testing.T) {
	llm, err := NewGeminiClient(context.Background(), "mock-api-key", "gemini-test-model")

	require.NoError(t, err)
	require.NotNil(t, llm)

	gemini, ok := llm.(*geminiClient)
	require.True(t, ok)
	require.Equal(t, "gemini-test-model", gemini.model)
}

func TestGeminiClient_GenerateResponse_Success(t *testing.T) {
	geminiAPIClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL.Path, "gemini-test-model:generateContent")
			require.Equal(t, "mock-api-key", req.Header.Get("x-goog-api-key"))

			return mockResponse(http.StatusOK, `{
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
			}`), nil
		},
	))

	llm := &geminiClient{
		client: geminiAPIClient,
		model:  "gemini-test-model",
	}

	response, err := llm.GenerateResponse(context.Background(), entity.LLMRequest{
		Messages: []entity.LLMMessage{
			{Role: "user", Content: "Get my booking"},
		},
		Tools: []entity.Tool{
			{
				Name:        "get_booking",
				Description: "Get booking details",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "Here are your booking details.", response.Text)
	require.Len(t, response.FunctionCalls, 1)
	require.Equal(t, "call-123", response.FunctionCalls[0].ID)
	require.Equal(t, "get_booking", response.FunctionCalls[0].Name)
	require.Equal(t, "booking-123", response.FunctionCalls[0].Arguments["booking_id"])
}

func TestGeminiClient_GenerateResponse_GenerateContentError(t *testing.T) {
	geminiAPIClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return mockResponse(
				http.StatusBadRequest,
				`{"error":{"code":400,"message":"Invalid request","status":"INVALID_ARGUMENT"}}`,
			), nil
		},
	))

	llm := &geminiClient{
		client: geminiAPIClient,
		model:  "gemini-test-model",
	}

	response, err := llm.GenerateResponse(context.Background(), entity.LLMRequest{
		Messages: []entity.LLMMessage{
			{Role: "user", Content: "Hello"},
		},
	})

	require.Nil(t, response)
	require.Error(t, err)
}

func TestGeminiClient_GenerateResponse_HTTPError(t *testing.T) {
	geminiAPIClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("mock network error")
		},
	))

	llm := &geminiClient{
		client: geminiAPIClient,
		model:  "gemini-test-model",
	}

	response, err := llm.GenerateResponse(context.Background(), entity.LLMRequest{
		Messages: []entity.LLMMessage{
			{Role: "user", Content: "Hello"},
		},
	})

	require.Nil(t, response)
	require.Error(t, err)
}

func TestConvertMessages(t *testing.T) {
	contents, err := convertMessages([]entity.LLMMessage{
		{Role: "user", Content: "Get my booking"},
		{
			Role: "assistant",
			FunctionCalls: []entity.FunctionCall{
				{
					ID:        "call-123",
					Name:      "get_booking",
					Arguments: map[string]any{"booking_id": "booking-123"},
				},
			},
		},
		{
			Role: "user",
			FunctionResponses: []entity.FunctionResponse{
				{
					ID:       "call-123",
					Name:     "get_booking",
					Response: map[string]any{"status": "CONFIRMED"},
				},
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, contents, 3)
	require.Equal(t, "user", contents[0].Role)
	require.Equal(t, "model", contents[1].Role)
	require.NotNil(t, contents[1].Parts[0].FunctionCall)
	require.NotNil(t, contents[2].Parts[0].FunctionResponse)
}

func TestConvertMessages_UnsupportedRole(t *testing.T) {
	contents, err := convertMessages([]entity.LLMMessage{
		{Role: "system", Content: "You are a helpful assistant"},
	})

	require.Nil(t, contents)
	require.Error(t, err)
}

func TestConvertTools(t *testing.T) {
	tools := convertTools([]entity.Tool{
		{
			Name:        "get_booking",
			Description: "Get booking details",
			InputSchema: map[string]any{"type": "object"},
		},
	})

	require.Len(t, tools, 1)
	require.Len(t, tools[0].FunctionDeclarations, 1)
	require.Equal(t, "get_booking", tools[0].FunctionDeclarations[0].Name)
	require.Nil(t, convertTools(nil))
}
