package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/util"
)

func RegisterBookingTools(
	server *mcp.Server,
	apiGateway client.APIGatewayClient,
) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.GetBookingTool.Name,
			Description: entity.GetBookingTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			input entity.GetBookingInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			body, err := util.GetBookingRequest(input)
			if err != nil {
				return nil, nil, err
			}

			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.GetBookingTool.Name,
				body,
			)
			return nil, output, err
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.ListUpcomingBookingsTool.Name,
			Description: entity.ListUpcomingBookingsTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			_ entity.ListUpcomingBookingsInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.ListUpcomingBookingsTool.Name,
				map[string]any{},
			)
			return nil, output, err
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        entity.ListPastBookingsTool.Name,
			Description: entity.ListPastBookingsTool.Description,
		},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			_ entity.ListPastBookingsInput,
		) (*mcp.CallToolResult, map[string]any, error) {
			output, err := apiGateway.ExecuteTool(
				ctx,
				entity.ListPastBookingsTool.Name,
				map[string]any{},
			)
			return nil, output, err
		},
	)
}
