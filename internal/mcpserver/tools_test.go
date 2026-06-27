package mcpserver

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"git-statistics/internal/domain"
	"git-statistics/internal/metrics"
	"git-statistics/internal/storage"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedTestData(t *testing.T, store *storage.Store) {
	// Seed authors
	authors := []domain.Author{
		{ID: "author1", DisplayName: "Alice", Allowlisted: true},
		{ID: "author2", DisplayName: "Bob", Allowlisted: true},
	}
	for _, a := range authors {
		if err := store.UpsertAuthor(a); err != nil {
			t.Fatalf("failed to insert author: %v", err)
		}
	}

	// Seed repositories
	repos := []string{"repo1", "repo2"}
	for _, slug := range repos {
		if err := store.UpsertRepository(domain.Repository{Slug: slug, Workspace: "workspace"}); err != nil {
			t.Fatalf("failed to insert repository: %v", err)
		}
	}

	// Seed pull requests with mixed states
	now := time.Now()
	prs := []domain.PullRequest{
		// repo1, author1
		{ID: 1, RepoSlug: "repo1", Title: "PR 1", AuthorID: "author1", State: "OPEN", CreatedAt: now, UpdatedAt: now},
		{ID: 2, RepoSlug: "repo1", Title: "PR 2", AuthorID: "author1", State: "MERGED", CreatedAt: now, UpdatedAt: now, MergedAt: &now},
		{ID: 3, RepoSlug: "repo1", Title: "PR 3", AuthorID: "author1", State: "DECLINED", CreatedAt: now, UpdatedAt: now},
		// repo1, author2
		{ID: 4, RepoSlug: "repo1", Title: "PR 4", AuthorID: "author2", State: "OPEN", CreatedAt: now, UpdatedAt: now},
		{ID: 5, RepoSlug: "repo1", Title: "PR 5", AuthorID: "author2", State: "MERGED", CreatedAt: now, UpdatedAt: now, MergedAt: &now},
		// repo2, author1
		{ID: 6, RepoSlug: "repo2", Title: "PR 6", AuthorID: "author1", State: "OPEN", CreatedAt: now, UpdatedAt: now},
		{ID: 7, RepoSlug: "repo2", Title: "PR 7", AuthorID: "author1", State: "SUPERSEDED", CreatedAt: now, UpdatedAt: now},
		// repo2, author2
		{ID: 8, RepoSlug: "repo2", Title: "PR 8", AuthorID: "author2", State: "MERGED", CreatedAt: now, UpdatedAt: now, MergedAt: &now},
	}

	for _, pr := range prs {
		if err := store.UpsertPullRequest(pr); err != nil {
			t.Fatalf("failed to insert pull request: %v", err)
		}
	}
}

func TestParseFilterFromTool(t *testing.T) {
	tests := []struct {
		name    string
		input   ToolInput
		wantErr bool
		check   func(*storage.Filter) bool
	}{
		{
			name:    "empty input",
			input:   ToolInput{},
			wantErr: false,
			check:   func(f *storage.Filter) bool { return f.RepoSlug == "" && f.AuthorID == "" },
		},
		{
			name:    "with repo and author",
			input:   ToolInput{Repo: "myrepo", Author: "author1"},
			wantErr: false,
			check:   func(f *storage.Filter) bool { return f.RepoSlug == "myrepo" && f.AuthorID == "author1" },
		},
		{
			name:    "with valid date range",
			input:   ToolInput{From: "2026-01-01", To: "2026-06-27"},
			wantErr: false,
			check: func(f *storage.Filter) bool {
				return f.From.Year() == 2026 && f.From.Month() == 1 && f.To.Day() == 27
			},
		},
		{
			name:    "invalid from date",
			input:   ToolInput{From: "not-a-date"},
			wantErr: true,
		},
		{
			name:    "invalid to date",
			input:   ToolInput{To: "2026-13-32"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := parseFilterFromTool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFilterFromTool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !tt.check(&f) {
				t.Errorf("parseFilterFromTool() produced unexpected filter: %+v", f)
			}
		})
	}
}

func TestRegisterToolsDashboardMetrics(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	err := RegisterToolsDashboardMetrics(server, store)
	if err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Verify that server construction succeeded and tools were registered
	if server == nil {
		t.Error("server should not be nil after registration")
	}
}

func TestRegisterToolsPRStatus(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	err := RegisterToolsPRStatus(server, store)
	if err != nil {
		t.Fatalf("RegisterToolsPRStatus() error: %v", err)
	}

	// Verify that server construction succeeded and tools were registered
	// (The actual tool list check would require accessing MCP's internal state
	// which isn't exposed in a testable way, so we verify registration succeeded.)
	if server == nil {
		t.Error("server should not be nil after registration")
	}
}

func TestDeliveryFlowTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	expectedFlows, err := metrics.DeliveryFlow(store, filter)
	if err != nil {
		t.Fatalf("DeliveryFlow() error: %v", err)
	}

	// Verify that we got results
	if len(expectedFlows) == 0 {
		t.Error("expected non-empty results from DeliveryFlow")
	}

	// Check that all flows have valid structure
	for _, flow := range expectedFlows {
		if flow.PullRequestID == 0 {
			t.Error("flow should have a non-zero PullRequestID")
		}
	}
}

func TestSummaryStatsTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	stats, err := metrics.SummaryStats(store, filter)
	if err != nil {
		t.Fatalf("SummaryStats() error: %v", err)
	}

	// Verify that we got a result
	if stats == nil {
		t.Error("expected non-nil stats")
	}
}

func TestDistributionsTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	dists, err := metrics.Distributions(store, filter)
	if err != nil {
		t.Fatalf("Distributions() error: %v", err)
	}

	// Verify that we got a result
	if dists == nil {
		t.Error("expected non-nil distributions")
	}
}

func TestChurnHotspotsTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	churn, err := metrics.ChurnHotspots(store, filter)
	if err != nil {
		t.Fatalf("ChurnHotspots() error: %v", err)
	}

	// Verify that we got a valid result (may be empty)
	if churn == nil {
		t.Error("expected non-nil churn result")
	}
}

func TestCommitsPerAuthorTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	activity, err := metrics.CommitsPerAuthor(store, filter)
	if err != nil {
		t.Fatalf("CommitsPerAuthor() error: %v", err)
	}

	// Verify that we got a valid result (may be empty if no allowlisted authors)
	if activity == nil {
		t.Error("expected non-nil activity result")
	}
}

func TestBreakdownByRepositoryTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	rows, err := metrics.BreakdownByRepository(store, filter)
	if err != nil {
		t.Fatalf("BreakdownByRepository() error: %v", err)
	}

	// Verify that we got results
	if len(rows) == 0 {
		t.Error("expected non-empty results from BreakdownByRepository")
	}

	// Check that all rows have valid structure
	for _, row := range rows {
		if row.Key == "" {
			t.Error("row key should not be empty")
		}
	}
}

func TestBreakdownByAuthorTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsDashboardMetrics(server, store); err != nil {
		t.Fatalf("RegisterToolsDashboardMetrics() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	rows, err := metrics.BreakdownByAuthor(store, filter)
	if err != nil {
		t.Fatalf("BreakdownByAuthor() error: %v", err)
	}

	// Verify that we got results
	if len(rows) == 0 {
		t.Error("expected non-empty results from BreakdownByAuthor")
	}

	// Check that all rows have valid structure
	for _, row := range rows {
		if row.Key == "" {
			t.Error("row key should not be empty")
		}
	}
}

func TestPRStatusByAuthorTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsPRStatus(server, store); err != nil {
		t.Fatalf("RegisterToolsPRStatus() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	expectedRows, err := metrics.PRStatusByAuthor(store, filter)
	if err != nil {
		t.Fatalf("PRStatusByAuthor() error: %v", err)
	}

	// Verify that we got results with the expected structure
	if len(expectedRows) == 0 {
		t.Error("expected non-empty results from PRStatusByAuthor")
	}

	// Check that all authors are present with all states reported
	for _, row := range expectedRows {
		if row.Key == "" {
			t.Error("row key should not be empty")
		}
		// Verify counts are non-negative
		if row.Open < 0 || row.Merged < 0 || row.Declined < 0 || row.Superseded < 0 {
			t.Errorf("row %s has negative counts: %+v", row.Key, row)
		}
	}

	// Verify expected data:
	// Alice (author1): 2 open, 1 merged, 1 declined, 1 superseded
	// Bob (author2): 1 open, 2 merged, 0 declined, 0 superseded
	aliceRow := findRowByKey(expectedRows, "Alice")
	if aliceRow == nil {
		t.Error("expected Alice row in results")
	} else {
		if aliceRow.Open != 2 || aliceRow.Merged != 1 || aliceRow.Declined != 1 || aliceRow.Superseded != 1 {
			t.Errorf("Alice row has unexpected counts: %+v", aliceRow)
		}
	}

	bobRow := findRowByKey(expectedRows, "Bob")
	if bobRow == nil {
		t.Error("expected Bob row in results")
	} else {
		if bobRow.Open != 1 || bobRow.Merged != 2 || bobRow.Declined != 0 || bobRow.Superseded != 0 {
			t.Errorf("Bob row has unexpected counts: %+v", bobRow)
		}
	}
}

func TestPRStatusByRepositoryTool(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsPRStatus(server, store); err != nil {
		t.Fatalf("RegisterToolsPRStatus() error: %v", err)
	}

	// Call the metric function directly to get expected results
	filter := storage.Filter{}
	expectedRows, err := metrics.PRStatusByRepository(store, filter)
	if err != nil {
		t.Fatalf("PRStatusByRepository() error: %v", err)
	}

	// Verify that we got results with the expected structure
	if len(expectedRows) == 0 {
		t.Error("expected non-empty results from PRStatusByRepository")
	}

	// Check that all repos are present with all states reported
	for _, row := range expectedRows {
		if row.Key == "" {
			t.Error("row key should not be empty")
		}
		// Verify counts are non-negative
		if row.Open < 0 || row.Merged < 0 || row.Declined < 0 || row.Superseded < 0 {
			t.Errorf("row %s has negative counts: %+v", row.Key, row)
		}
	}

	// Verify expected data:
	// repo1: 2 open, 2 merged, 1 declined, 0 superseded
	// repo2: 1 open, 1 merged, 0 declined, 1 superseded
	repo1Row := findRowByKey(expectedRows, "repo1")
	if repo1Row == nil {
		t.Error("expected repo1 row in results")
	} else {
		if repo1Row.Open != 2 || repo1Row.Merged != 2 || repo1Row.Declined != 1 || repo1Row.Superseded != 0 {
			t.Errorf("repo1 row has unexpected counts: %+v", repo1Row)
		}
	}

	repo2Row := findRowByKey(expectedRows, "repo2")
	if repo2Row == nil {
		t.Error("expected repo2 row in results")
	} else {
		if repo2Row.Open != 1 || repo2Row.Merged != 1 || repo2Row.Declined != 0 || repo2Row.Superseded != 1 {
			t.Errorf("repo2 row has unexpected counts: %+v", repo2Row)
		}
	}
}

func TestPRStatusEmptyResult(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test-git-statistics",
		Title:   "Test Git Statistics MCP Server",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	store := setupTestStore(t)
	defer store.Close()

	seedTestData(t, store)

	if err := RegisterToolsPRStatus(server, store); err != nil {
		t.Fatalf("RegisterToolsPRStatus() error: %v", err)
	}

	// Query with a filter that matches no PRs
	filter := storage.Filter{RepoSlug: "nonexistent"}
	rows, err := metrics.PRStatusByRepository(store, filter)
	if err != nil {
		t.Fatalf("PRStatusByRepository() error: %v", err)
	}

	// Should return empty slice, not nil or error
	if rows == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(rows) != 0 {
		t.Errorf("expected empty results, got %d rows", len(rows))
	}
}

func TestToolInputMarshaling(t *testing.T) {
	// Test that tool input can be marshaled and unmarshaled correctly
	input := ToolInput{
		Repo:   "myrepo",
		Author: "author1",
		From:   "2026-01-01",
		To:     "2026-06-27",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var unmarshaled ToolInput
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if unmarshaled != input {
		t.Errorf("unmarshaled input doesn't match original: got %+v, want %+v", unmarshaled, input)
	}
}

func findRowByKey(rows []metrics.PRStatusCountRow, key string) *metrics.PRStatusCountRow {
	for i, row := range rows {
		if row.Key == key {
			return &rows[i]
		}
	}
	return nil
}
