package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/metadata"
)

//go:generate mockgen -source=mcp.go -destination=../../../gen/mocks/chatbot/client/mcp_mock.go -package=mocks

type MCPClient interface {
	ListTools(ctx context.Context) ([]entity.Tool, error)
	CallTool(ctx context.Context, name string, arguments map[string]any) (any, error)
}

type client struct {
	endpoint string
	client   *mcp.Client
}

func NewMCPClient(endpoint string, mcpClient *mcp.Client) MCPClient {
	return &client{
		endpoint: endpoint,
		client:   mcpClient,
	}
}

type identityTransport struct {
	base http.RoundTripper
}

func (t *identityTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	request := req.Clone(req.Context())
	request.Header = req.Header.Clone()

	if md, ok := metadata.FromIncomingContext(req.Context()); ok {
		if userIDs := md.Get("x-user-id"); len(userIDs) > 0 {
			request.Header.Set("X-User-ID", userIDs[0])
		}

		if roles := md.Get("x-user-role"); len(roles) > 0 {
			request.Header.Set("X-User-Role", roles[0])
		}
	}

	return t.base.RoundTrip(request)
}

func (c *client) connect(ctx context.Context) (*mcp.ClientSession, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: c.endpoint,
		HTTPClient: &http.Client{
			Transport: &identityTransport{
				base: http.DefaultTransport,
			},
		},
		DisableStandaloneSSE: true,
	}

	return c.client.Connect(ctx, transport, nil)
}

func (c *client) ListTools(ctx context.Context) ([]entity.Tool, error) {
	session, err := c.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	var tools []entity.Tool
	cursor := ""

	for {
		result, err := session.ListTools(ctx, &mcp.ListToolsParams{
			Cursor: cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}

		for _, tool := range result.Tools {
			schema, err := toMap(tool.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("decode input schema for tool %q: %w", tool.Name, err)
			}

			tools = append(tools, entity.Tool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schema,
			})
		}

		if result.NextCursor == "" {
			break
		}

		cursor = result.NextCursor
	}

	return tools, nil
}

func (c *client) CallTool(ctx context.Context, name string, arguments map[string]any) (any, error) {
	session, err := c.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("call MCP tool %q: %w", name, err)
	}

	return result, nil
}

func toMap(value any) (map[string]any, error) {
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
