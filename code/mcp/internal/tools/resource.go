package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/util"
)

func RegisterResourceTools(
	server *mcp.Server,
	apiGateway client.APIGatewayClient,
) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.ListResourceTypesTool.Name,
			Description: entity.ListResourceTypesTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			_ entity.ListResourceTypesInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.ListResourceTypesTool.Name,
				map[string]any{},
			)
			return nil, output, err
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.SearchResourcesTool.Name,
			Description: entity.SearchResourcesTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			input entity.SearchResourcesInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			body, err := util.SearchResourcesRequest(input)
			if err != nil {
				return nil, nil, err
			}

			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.SearchResourcesTool.Name,
				body,
			)
			return nil, output, err
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.GetResourceTool.Name,
			Description: entity.GetResourceTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			input entity.GetResourceInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			body, err := util.GetResourceRequest(input)
			if err != nil {
				return nil, nil, err
			}

			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.GetResourceTool.Name,
				body,
			)
			return nil, output, err
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.GetResourceAvailabilityTool.Name,
			Description: entity.GetResourceAvailabilityTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			input entity.GetResourceAvailabilityInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			body, err := util.GetResourceAvailabilityRequest(input)
			if err != nil {
				return nil, nil, err
			}

			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.GetResourceAvailabilityTool.Name,
				body,
			)
			return nil, output, err
		},
	)
}
