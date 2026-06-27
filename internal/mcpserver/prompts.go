package mcpserver

import (
	"context"
	"errors"
	"fmt"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPrompts registers the three curated MCP prompts that bundle tool calls into
// investigation workflows: investigate_progress, team_activity_summary, and pr_health_check.
func RegisterPrompts(server *mcp.Server) error {
	if server == nil {
		return errors.New("server must not be nil")
	}

	// 1. investigate_progress prompt
	// Guides the agent through PR status, delivery flow, and commit activity for a repo/author scope
	investigateProgressPrompt := &mcp.Prompt{
		Name:        "investigate_progress",
		Description: "Investigate feature progress by examining PR status, delivery flow, and recent author activity for a specific repository or repository and author.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "repo",
				Description: "The repository slug to investigate (required)",
				Required:    true,
			},
			{
				Name:        "author",
				Description: "Optional author ID to narrow investigation to a specific author's PRs",
				Required:    false,
			},
		},
	}

	investigateProgressHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		repo := req.Params.Arguments["repo"]
		if repo == "" {
			return nil, errors.New("repo argument is required")
		}
		author := req.Params.Arguments["author"]

		// Build the prompt message that guides the agent through tool calls
		var toolCalls string
		if author != "" {
			toolCalls = fmt.Sprintf(
				`To investigate progress in repository '%s' for author '%s':

1. First, call **pr_status_by_author** with:
   - repo: "%s"
   - author: "%s"
   This shows the count of open, merged, declined, and superseded PRs for this author.

2. Next, call **delivery_flow** with:
   - repo: "%s"
   - author: "%s"
   This shows PR cycle time, time-to-first-review, and time-in-review metrics for merged PRs.

3. Finally, call **commits_per_author** with:
   - repo: "%s"
   - author: "%s"
   This shows recent commit activity for this author.

Use these tool calls in order to build a complete picture of this author's progress in the repository.`,
				repo, author, repo, author, repo, author, repo, author,
			)
		} else {
			toolCalls = fmt.Sprintf(
				`To investigate progress in repository '%s':

1. First, call **pr_status_by_repo** with:
   - repo: "%s"
   This shows the count of open, merged, declined, and superseded PRs across all authors in this repo.

2. Next, call **delivery_flow** with:
   - repo: "%s"
   This shows PR cycle time, time-to-first-review, and time-in-review metrics for merged PRs in this repo.

3. Finally, call **commits_per_author** with:
   - repo: "%s"
   This shows recent commit activity across all authors in the repository.

4. Optionally, call **breakdown_by_author** with:
   - repo: "%s"
   This shows PR metrics broken down by author to identify variation across the team.

Use these tool calls in order to build a complete picture of repository progress.`,
				repo, repo, repo, repo, repo,
			)
		}

		return &mcp.GetPromptResult{
			Description: investigateProgressPrompt.Description,
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: toolCalls},
				},
			},
		}, nil
	}

	server.AddPrompt(investigateProgressPrompt, investigateProgressHandler)

	// 2. team_activity_summary prompt
	// Guides the agent through commit activity and author/repo breakdowns
	teamActivityPrompt := &mcp.Prompt{
		Name:        "team_activity_summary",
		Description: "Get a summary of team activity including commit counts per author and PR metrics by repository or author.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "repo",
				Description: "Optional repository slug to narrow activity to a specific repo",
				Required:    false,
			},
			{
				Name:        "from",
				Description: "Optional start date in YYYY-MM-DD format",
				Required:    false,
			},
			{
				Name:        "to",
				Description: "Optional end date in YYYY-MM-DD format",
				Required:    false,
			},
		},
	}

	teamActivityHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		repo := req.Params.Arguments["repo"]
		from := req.Params.Arguments["from"]
		to := req.Params.Arguments["to"]

		// Build the prompt message
		var toolCalls string
		if repo != "" {
			toolCalls = fmt.Sprintf(
				`To summarize team activity in repository '%s':

1. First, call **commits_per_author** with:
   - repo: "%s"
   - from: "%s"
   - to: "%s"
   This shows commit counts per author in this repository.

2. Next, call **breakdown_by_author** with:
   - repo: "%s"
   - from: "%s"
   - to: "%s"
   This shows PR cycle time and review metrics broken down by author.

3. Finally, call **pr_status_by_author** with:
   - repo: "%s"
   - from: "%s"
   - to: "%s"
   This shows PR state distribution across authors.

Use these tools to get a complete view of team activity in this repository.`,
				repo,
				repo, from, to,
				repo, from, to,
				repo, from, to,
			)
		} else {
			toolCalls = fmt.Sprintf(
				`To summarize team activity across all repositories:

1. First, call **commits_per_author** with:
   - from: "%s"
   - to: "%s"
   This shows commit counts per author across all repos.

2. Next, call **breakdown_by_author** with:
   - from: "%s"
   - to: "%s"
   This shows PR cycle time and review metrics broken down by author.

3. Then, call **breakdown_by_repository** with:
   - from: "%s"
   - to: "%s"
   This shows PR metrics broken down by repository.

4. Finally, call **pr_status_by_author** with:
   - from: "%s"
   - to: "%s"
   This shows PR state distribution across authors.

Use these tools to get a complete picture of team activity.`,
				from, to,
				from, to,
				from, to,
				from, to,
			)
		}

		return &mcp.GetPromptResult{
			Description: teamActivityPrompt.Description,
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: toolCalls},
				},
			},
		}, nil
	}

	server.AddPrompt(teamActivityPrompt, teamActivityHandler)

	// 3. pr_health_check prompt
	// Guides the agent through delivery flow summary, distributions, and PR status breakdown
	prHealthCheckPrompt := &mcp.Prompt{
		Name:        "pr_health_check",
		Description: "Perform a health check on PR metrics for a repository, including summary statistics, percentile distributions, and PR state breakdown.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "repo",
				Description: "The repository slug to check (required)",
				Required:    true,
			},
		},
	}

	prHealthCheckHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		repo := req.Params.Arguments["repo"]
		if repo == "" {
			return nil, errors.New("repo argument is required")
		}

		// Build the prompt message
		toolCalls := fmt.Sprintf(
			`To perform a health check on PR metrics for repository '%s':

1. First, call **summary_stats** with:
   - repo: "%s"
   This shows aggregate statistics (average, median, min, max cycle time, review times).

2. Next, call **distributions** with:
   - repo: "%s"
   This shows percentile distributions (P50, P75, P90, P95, P99) for cycle time and review times.

3. Finally, call **pr_status_by_repo** with:
   - repo: "%s"
   This shows the distribution of PR states (open, merged, declined, superseded).

Interpret the results to identify:
- Whether median cycle time is reasonable
- Whether review time percentiles indicate bottlenecks
- The ratio of merged to open/declined PRs
- Any outliers in the distribution that may need investigation

Use these tools in order to assess overall PR health.`,
			repo,
			repo,
			repo,
			repo,
		)

		return &mcp.GetPromptResult{
			Description: prHealthCheckPrompt.Description,
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: toolCalls},
				},
			},
		}, nil
	}

	server.AddPrompt(prHealthCheckPrompt, prHealthCheckHandler)

	return nil
}
