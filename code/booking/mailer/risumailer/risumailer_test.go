package risumailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNew(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id")

	require.NotNil(t, s)

	sender, ok := s.(*sender)
	require.True(t, ok)

	require.Equal(t, "mock-api-key", sender.apiKey)
	require.Equal(t, "mock-sender-id", sender.senderEmailID)
	require.NotNil(t, sender.httpClient)
}

func TestSender_Send_ContextCancelled(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Send(
		ctx,
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.ErrorIs(t, err, context.Canceled)
}

func TestSender_Send_Success(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id").(*sender)

	s.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, apiURL, req.URL.String())
		require.Equal(t, "Bearer mock-api-key", req.Header.Get("Authorization"))
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))

		var requestBody emailRequest
		err := json.NewDecoder(req.Body).Decode(&requestBody)
		require.NoError(t, err)

		require.Equal(t, []string{"mock-recipient@example.com"}, requestBody.To)
		require.Equal(t, "Test Subject", requestBody.Subject)
		require.Equal(t, "<h1>Hello</h1>", requestBody.HTML)
		require.Equal(t, "mock-sender-id", requestBody.SenderEmailID)

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"success":true,"data":{"id":"mock-mail-id","status":"QUEUED"}}`)),
			Header:     make(http.Header),
		}, nil
	})

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.NoError(t, err)
}

func TestSender_SendAPIError(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id").(*sender)

	s.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body: io.NopCloser(bytes.NewBufferString(
				`{"success":false,"error":"SENDER_NOT_VERIFIED"}`,
			)),
			Header: make(http.Header),
		}, nil
	})

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "status=403")
	require.Contains(t, err.Error(), "SENDER_NOT_VERIFIED")
}

func TestSender_SendHTTPError(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id").(*sender)

	s.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("mock network error")
	})

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "risu send: http request")
	require.Contains(t, err.Error(), "mock network error")
}

func TestSender_SendInvalidJSONResponse(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id").(*sender)

	s.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`invalid-json`)),
			Header:     make(http.Header),
		}, nil
	})

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "decode response")
}

func TestSender_SendSuccessFalse(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id").(*sender)

	s.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"success":false}`)),
			Header:     make(http.Header),
		}, nil
	})

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "API returned success=false")
}

func TestSender_SendEmptyAPIKey(t *testing.T) {
	s := New("", "mock-sender-id")

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"mock-recipient@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "API key is empty")
}

func TestSender_SendEmptyRecipient(t *testing.T) {
	s := New("mock-api-key", "mock-sender-id")

	err := s.Send(
		context.Background(),
		"mock-from@example.com",
		"",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "recipient is empty")
}
