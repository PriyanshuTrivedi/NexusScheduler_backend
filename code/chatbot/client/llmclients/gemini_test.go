package llmclients

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/util"
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

func TestNewGeminiLLM(t *testing.T) {
	geminiClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return mockResponse(http.StatusOK, `{}`), nil
		},
	))

	llm := NewGeminiLLM(geminiClient, "gemini-test-model")

	require.NotNil(t, llm)

	gemini, ok := llm.(*geminiLLM)
	require.True(t, ok)

	require.Equal(t, geminiClient, gemini.client)
	require.Equal(t, "gemini-test-model", gemini.model)
}

func TestGeminiLLM_GenerateReply_Success(t *testing.T) {
	geminiClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.True(t, strings.Contains(req.URL.Path, "gemini-test-model:generateContent"))
			require.Equal(t, "mock-api-key", req.Header.Get("x-goog-api-key"))

			return mockResponse(
				http.StatusOK,
				`{
					"candidates": [
						{
							"content": {
								"role": "model",
								"parts": [
									{"text": "Hi there!"}
								]
							},
							"finishReason": "STOP"
						}
					]
				}`,
			), nil
		},
	))

	llm := NewGeminiLLM(geminiClient, "gemini-test-model")

	reply, err := llm.GenerateReply(context.Background(), []entity.Message{
		{
			Role:    entity.RoleUser,
			Content: "Hello",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "Hi there!", reply)
}

func TestGeminiLLM_GenerateReply_GenerateContentError(t *testing.T) {
	geminiClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return mockResponse(
				http.StatusBadRequest,
				`{"error":{"code":400,"message":"Invalid request","status":"INVALID_ARGUMENT"}}`,
			), nil
		},
	))

	llm := NewGeminiLLM(geminiClient, "gemini-test-model")

	reply, err := llm.GenerateReply(context.Background(), []entity.Message{
		{
			Role:    entity.RoleUser,
			Content: "Hello",
		},
	})

	require.Empty(t, reply)
	require.ErrorIs(t, err, ErrGenerateContent)
}

func TestGeminiLLM_GenerateReply_UnsupportedMessageRole(t *testing.T) {
	called := false

	geminiClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			called = true
			return mockResponse(http.StatusOK, `{}`), nil
		},
	))

	llm := NewGeminiLLM(geminiClient, "gemini-test-model")

	reply, err := llm.GenerateReply(context.Background(), []entity.Message{
		{
			Role:    entity.Role("SYSTEM"),
			Content: "You are a helpful assistant",
		},
	})

	require.Empty(t, reply)
	require.ErrorIs(t, err, util.ErrUnsupportedMessageRole)
	require.False(t, called)
}

func TestGeminiLLM_GenerateReply_EmptyResponse(t *testing.T) {
	geminiClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return mockResponse(http.StatusOK, `{}`), nil
		},
	))

	llm := NewGeminiLLM(geminiClient, "gemini-test-model")

	reply, err := llm.GenerateReply(context.Background(), []entity.Message{
		{
			Role:    entity.RoleUser,
			Content: "Hello",
		},
	})

	require.Empty(t, reply)
	require.ErrorIs(t, err, util.ErrEmptyGeneratedReply)
}

func TestGeminiLLM_GenerateReply_HTTPError(t *testing.T) {
	geminiClient := newMockGeminiClient(roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("mock network error")
		},
	))

	llm := NewGeminiLLM(geminiClient, "gemini-test-model")

	reply, err := llm.GenerateReply(context.Background(), []entity.Message{
		{
			Role:    entity.RoleUser,
			Content: "Hello",
		},
	})

	require.Empty(t, reply)
	require.ErrorIs(t, err, ErrGenerateContent)
}
