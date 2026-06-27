package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"git-statistics/internal/metrics"
	"git-statistics/internal/storage"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolInput is the input struct for all tool functions.
// It mirrors storage.Filter with string date fields.
type ToolInput struct {
	Repo   string `json:"repo"`
	Author string `json:"author"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// Output wrapper types for tools (MCP requires output schema to be an object)
type DeliveryFlowOutput struct {
	Flows []metrics.PullRequestFlow `json:"flows"`
}

type SummaryStatsOutput struct {
	Stats *metrics.PRSummaryStats `json:"stats"`
}

type DistributionsOutput struct {
	Distributions *metrics.DistributionMetrics `json:"distributions"`
}

type ChurnHotspotsOutput struct {
	Hotspots []metrics.FileChurn `json:"hotspots"`
}

type CommitsPerAuthorOutput struct {
	Activity []metrics.AuthorActivity `json:"activity"`
}

type BreakdownByRepositoryOutput struct {
	Rows []metrics.BreakdownRow `json:"rows"`
}

type BreakdownByAuthorOutput struct {
	Rows []metrics.BreakdownRow `json:"rows"`
}

type PRStatusByAuthorOutput struct {
	Rows []metrics.PRStatusCountRow `json:"rows"`
}

type PRStatusByRepositoryOutput struct {
	Rows []metrics.PRStatusCountRow `json:"rows"`
}

// parseFilterFromTool parses filter parameters from a tool input struct.
// Date strings should be in "2006-01-02" format.
func parseFilterFromTool(input ToolInput) (storage.Filter, error) {
	f := storage.Filter{
		RepoSlug: input.Repo,
		AuthorID: input.Author,
	}

	if input.From != "" {
		t, err := time.Parse("2006-01-02", input.From)
		if err != nil {
			return storage.Filter{}, fmt.Errorf("invalid 'from' date format: %w", err)
		}
		f.From = t
	}

	if input.To != "" {
		t, err := time.Parse("2006-01-02", input.To)
		if err != nil {
			return storage.Filter{}, fmt.Errorf("invalid 'to' date format: %w", err)
		}
		f.To = t
	}

	return f, nil
}

// RegisterToolsDashboardMetrics registers the seven MCP tools for existing dashboard metrics.
func RegisterToolsDashboardMetrics(server *mcp.Server, store *storage.Store) error {
	if server == nil || store == nil {
		return errors.New("server and store must not be nil")
	}

	// 1. DeliveryFlow tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delivery_flow",
		Description: "Returns PR cycle time, time-to-first-review, and time-in-review for merged pull requests, optionally filtered by repo/author/date range — same data as the Delivery Flow dashboard.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, DeliveryFlowOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, DeliveryFlowOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		flows, err := metrics.DeliveryFlow(store, filter)
		if err != nil {
			return nil, DeliveryFlowOutput{}, fmt.Errorf("querying delivery flow: %w", err)
		}

		return nil, DeliveryFlowOutput{Flows: flows}, nil
	})

	// 2. SummaryStats tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "summary_stats",
		Description: "Returns aggregate PR timing statistics (average, median, min, max) for merged pull requests, optionally filtered by repo/author/date range — same data as the Delivery Flow summary panel.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, SummaryStatsOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, SummaryStatsOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		stats, err := metrics.SummaryStats(store, filter)
		if err != nil {
			return nil, SummaryStatsOutput{}, fmt.Errorf("querying summary stats: %w", err)
		}

		return nil, SummaryStatsOutput{Stats: stats}, nil
	})

	// 3. Distributions tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "distributions",
		Description: "Returns percentile distributions (P50/P75/P90/P95/P99) for PR cycle time, time-to-first-review, and time-in-review, optionally filtered by repo/author/date range — same data as the Delivery Flow distributions panel.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, DistributionsOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, DistributionsOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		dists, err := metrics.Distributions(store, filter)
		if err != nil {
			return nil, DistributionsOutput{}, fmt.Errorf("querying distributions: %w", err)
		}

		return nil, DistributionsOutput{Distributions: dists}, nil
	})

	// 4. ChurnHotspots tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "churn_hotspots",
		Description: "Returns files with the highest lines changed and commit frequency, optionally filtered by repo/author/date range — same data as the Code Churn dashboard.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, ChurnHotspotsOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, ChurnHotspotsOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		churn, err := metrics.ChurnHotspots(store, filter)
		if err != nil {
			return nil, ChurnHotspotsOutput{}, fmt.Errorf("querying churn hotspots: %w", err)
		}

		return nil, ChurnHotspotsOutput{Hotspots: churn}, nil
	})

	// 5. CommitsPerAuthor tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "commits_per_author",
		Description: "Returns commit counts for each allowlisted author, optionally filtered by repo/date range — same data as the Activity dashboard.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, CommitsPerAuthorOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, CommitsPerAuthorOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		activity, err := metrics.CommitsPerAuthor(store, filter)
		if err != nil {
			return nil, CommitsPerAuthorOutput{}, fmt.Errorf("querying commits per author: %w", err)
		}

		return nil, CommitsPerAuthorOutput{Activity: activity}, nil
	})

	// 6. BreakdownByRepository tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "breakdown_by_repository",
		Description: "Returns PR timing metrics (cycle time, time-to-first-review, time-in-review) grouped by repository, optionally filtered by author/date range — same data as the Delivery Flow repo breakdown.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, BreakdownByRepositoryOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, BreakdownByRepositoryOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		rows, err := metrics.BreakdownByRepository(store, filter)
		if err != nil {
			return nil, BreakdownByRepositoryOutput{}, fmt.Errorf("querying breakdown by repository: %w", err)
		}

		return nil, BreakdownByRepositoryOutput{Rows: rows}, nil
	})

	// 7. BreakdownByAuthor tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "breakdown_by_author",
		Description: "Returns PR timing metrics (cycle time, time-to-first-review, time-in-review) grouped by author, optionally filtered by repo/date range — same data as the Delivery Flow author breakdown.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, BreakdownByAuthorOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, BreakdownByAuthorOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		rows, err := metrics.BreakdownByAuthor(store, filter)
		if err != nil {
			return nil, BreakdownByAuthorOutput{}, fmt.Errorf("querying breakdown by author: %w", err)
		}

		return nil, BreakdownByAuthorOutput{Rows: rows}, nil
	})

	return nil
}

// RegisterToolsPRStatus registers the PR status breakdown tools with the MCP server.
func RegisterToolsPRStatus(server *mcp.Server, store *storage.Store) error {
	if server == nil || store == nil {
		return errors.New("server and store must not be nil")
	}

	// PR Status by Author tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "pr_status_by_author",
		Description: "Returns PR state counts (open, merged, declined, superseded) grouped by author display name. Optionally filtered by repository, date range.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, PRStatusByAuthorOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, PRStatusByAuthorOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		rows, err := metrics.PRStatusByAuthor(store, filter)
		if err != nil {
			return nil, PRStatusByAuthorOutput{}, fmt.Errorf("querying PR status by author: %w", err)
		}

		return nil, PRStatusByAuthorOutput{Rows: rows}, nil
	})

	// PR Status by Repository tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "pr_status_by_repo",
		Description: "Returns PR state counts (open, merged, declined, superseded) grouped by repository slug. Optionally filtered by author, date range.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, PRStatusByRepositoryOutput, error) {
		filter, err := parseFilterFromTool(input)
		if err != nil {
			return nil, PRStatusByRepositoryOutput{}, fmt.Errorf("parsing filter: %w", err)
		}

		rows, err := metrics.PRStatusByRepository(store, filter)
		if err != nil {
			return nil, PRStatusByRepositoryOutput{}, fmt.Errorf("querying PR status by repository: %w", err)
		}

		return nil, PRStatusByRepositoryOutput{Rows: rows}, nil
	})

	return nil
}
