package risumailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

//go:generate mockgen -source=risumailer.go -destination=../../../../gen/mocks/booking/mailer/risumailer/risumailer_mock.go -package=mocks

const apiURL = "https://risumail.risu.in/api/v1/emails"

type Sender interface {
	Send(ctx context.Context, from, to, subject, body string) error
}

type sender struct {
	apiKey        string
	senderEmailID string
	httpClient    *http.Client
}

type emailRequest struct {
	To            []string `json:"to"`
	Subject       string   `json:"subject"`
	HTML          string   `json:"html"`
	SenderEmailID string   `json:"senderEmailId,omitempty"`
}

type emailResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"data"`
}

func New(apiKey, senderEmailID string) Sender {
	return &sender{
		apiKey:        apiKey,
		senderEmailID: senderEmailID,
		httpClient:    &http.Client{},
	}
}

func (s *sender) Send(ctx context.Context, from, to, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if s.apiKey == "" {
		return fmt.Errorf("risu send: API key is empty")
	}

	if to == "" {
		return fmt.Errorf("risu send: recipient is empty")
	}

	reqBody := emailRequest{
		To:            []string{to},
		Subject:       subject,
		HTML:          body,
		SenderEmailID: s.senderEmailID,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("risu send: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("risu send: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("risu send: http request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("risu send: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"risu send: status=%d body=%s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	var result emailResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf(
			"risu send: decode response: %w",
			err,
		)
	}

	if !result.Success {
		return fmt.Errorf("risu send: API returned success=false")
	}

	return nil
}
