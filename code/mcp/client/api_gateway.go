package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
)

const executeToolPath = "/internal/v1/chat/tools/execute"

type APIGatewayClient interface {
	ExecuteTool(ctx context.Context, tool string, body map[string]any) (map[string]any, error)
}

type apiGatewayClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAPIGatewayClient(baseURL string, timeout time.Duration) (APIGatewayClient, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("api gateway: invalid base URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("api gateway: unsupported URL scheme %q", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("api gateway: base URL must include a host")
	}

	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &apiGatewayClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *apiGatewayClient) ExecuteTool(
	ctx context.Context,
	tool string,
	body map[string]any,
) (map[string]any, error) {
	identity, ok := entity.IdentityFromContext(ctx)
	if !ok {
		return nil, errors.New("api gateway: missing user identity in MCP request context")
	}

	payload := map[string]any{
		"tool": tool,
		"body": body,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("api gateway: encode request: %w", err)
	}

	endpoint, err := url.JoinPath(c.baseURL, executeToolPath)
	if err != nil {
		return nil, fmt.Errorf("api gateway: construct URL: %w", err)
	}
	fmt.Printf("endpoint url->%s\n", endpoint)
	fmt.Printf("req body->%v\n", body)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(encoded),
	)
	if err != nil {
		return nil, fmt.Errorf("api gateway: create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", identity.UserID)
	req.Header.Set("X-User-Role", identity.Role)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api gateway: request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("api gateway: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"api gateway: request failed with status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	if len(responseBody) == 0 {
		return map[string]any{}, nil
	}

	var output map[string]any
	if err := json.Unmarshal(responseBody, &output); err != nil {
		return nil, fmt.Errorf("api gateway: decode response: %w", err)
	}

	return output, nil
}
