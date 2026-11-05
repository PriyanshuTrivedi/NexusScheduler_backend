package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func newTestMCPSDKClient() *mcp.Client {
	return mcp.NewClient(
		&mcp.Implementation{
			Name:    "test-chatbot",
			Version: "1.0.0",
		},
		nil,
	)
}

func TestToMap(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    map[string]any
		wantErr bool
	}{
		{
			name:  "nil value",
			input: nil,
			want:  map[string]any{},
		},
		{
			name: "map value",
			input: map[string]any{
				"name":    "search_resources",
				"enabled": true,
			},
			want: map[string]any{
				"name":    "search_resources",
				"enabled": true,
			},
		},
		{
			name: "struct value",
			input: struct {
				Name    string `json:"name"`
				Enabled bool   `json:"enabled"`
			}{
				Name:    "search_resources",
				Enabled: true,
			},
			want: map[string]any{
				"name":    "search_resources",
				"enabled": true,
			},
		},
		{
			name: "map with nested values",
			input: map[string]any{
				"name": "search_resources",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
				},
			},
			want: map[string]any{
				"name": "search_resources",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
		{
			name:    "unsupported value",
			input:   make(chan int),
			wantErr: true,
		},
		{
			name:    "non-object JSON value",
			input:   []string{"one", "two"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toMap(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIdentityTransportRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		metadata   metadata.MD
		wantUserID string
		wantRole   string
	}{
		{
			name: "propagates user identity",
			metadata: metadata.Pairs(
				"x-user-id", "user-123",
				"x-user-role", "USER",
			),
			wantUserID: "user-123",
			wantRole:   "USER",
		},
		{
			name: "propagates first value when multiple values exist",
			metadata: metadata.Pairs(
				"x-user-id", "user-123",
				"x-user-id", "user-456",
				"x-user-role", "ADMIN",
				"x-user-role", "USER",
			),
			wantUserID: "user-123",
			wantRole:   "ADMIN",
		},
		{
			name:     "does not add identity headers when metadata is absent",
			metadata: metadata.MD{},
		},
		{
			name: "propagates only available metadata",
			metadata: metadata.Pairs(
				"x-user-id", "user-123",
			),
			wantUserID: "user-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantUserID, r.Header.Get("X-User-ID"))
				assert.Equal(t, tt.wantRole, r.Header.Get("X-User-Role"))

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			ctx := context.Background()
			if tt.metadata != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.metadata)
			}

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				server.URL,
				nil,
			)
			require.NoError(t, err)

			transport := &identityTransport{
				base: http.DefaultTransport,
			}

			resp, err := transport.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestIdentityTransportDoesNotMutateOriginalRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "user-123", r.Header.Get("X-User-ID"))
		assert.Equal(t, "ADMIN", r.Header.Get("X-User-Role"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(
			"x-user-id", "user-123",
			"x-user-role", "ADMIN",
		),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	req.Header.Set("X-Custom-Header", "original")

	transport := &identityTransport{
		base: http.DefaultTransport,
	}

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "", req.Header.Get("X-User-ID"))
	assert.Equal(t, "", req.Header.Get("X-User-Role"))
	assert.Equal(t, "original", req.Header.Get("X-Custom-Header"))
}

func TestNewMCPClient(t *testing.T) {
	mcpClient := NewMCPClient(
		"http://localhost:8080",
		newTestMCPSDKClient(),
	)

	require.NotNil(t, mcpClient)
}

func TestListToolsConnectError(t *testing.T) {
	mcpClient := NewMCPClient(
		"://invalid-url",
		newTestMCPSDKClient(),
	)

	_, err := mcpClient.ListTools(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "connect to MCP server")
}

func TestCallToolConnectError(t *testing.T) {
	mcpClient := NewMCPClient(
		"://invalid-url",
		newTestMCPSDKClient(),
	)

	_, err := mcpClient.CallTool(
		context.Background(),
		"search_resources",
		map[string]any{"name": "Arun"},
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "connect to MCP server")
}

func TestToMapJSONCompatibility(t *testing.T) {
	input := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}{
		Name:        "get_resource",
		Description: "Fetch a resource",
	}

	got, err := toMap(input)
	require.NoError(t, err)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	want := map[string]any{
		"name":        "get_resource",
		"description": "Fetch a resource",
	}

	assert.Equal(t, want, decoded)
}
