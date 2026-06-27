package main

import (
	"context"
	"git-statistics/internal/mcpserver"
	"git-statistics/internal/storage"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func runMCPServer(store *storage.Store) error {
	// Create a new MCP server with basic implementation info
	impl := &mcp.Implementation{
		Name:    "git-statistics",
		Title:   "Git Statistics MCP Server",
		Version: "0.1.0",
	}

	// Create server with no custom options
	server := mcp.NewServer(impl, nil)

	// Register tools (U3, U4, U5 add their registrations here)
	if err := mcpserver.RegisterToolsDashboardMetrics(server, store); err != nil {
		return err
	}
	if err := mcpserver.RegisterToolsPRStatus(server, store); err != nil {
		return err
	}

	// Register prompts (U5)
	if err := mcpserver.RegisterPrompts(server); err != nil {
		return err
	}

	// Run the server on stdio transport
	transport := &mcp.StdioTransport{}
	ctx := context.Background()
	return server.Run(ctx, transport)
}
