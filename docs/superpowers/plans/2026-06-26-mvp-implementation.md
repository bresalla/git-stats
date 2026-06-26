# Git Analytics App MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary that syncs commits/PRs/reviews from a configured set of Bitbucket Cloud repos into SQLite, computes Delivery Flow, Code Churn, and Activity metrics, and serves them as server-rendered dashboards — runnable entirely on a developer's laptop with `go run ./cmd/server`.

**Architecture:** Layered Go packages following the spec's pipeline: `bitbucket` (HTTP client) → `normalize` (raw → domain entities) → `storage` (SQLite upsert/query) → `ingest` (orchestrates client+normalize+storage with watermarks) → `metrics` (aggregates) → `web` (HTTP handlers + html/template). A `scheduler` ticks `ingest` on an interval; a manual "Sync now" button hits the same code path. Everything runs in one process, no external services.

**Tech Stack:** Go 1.22+, `modernc.org/sqlite` (pure-Go SQLite driver — no cgo, so `go build` works on Windows/Mac/Linux with no extra toolchain), `gopkg.in/yaml.v3` (config), `net/http` + `html/template` (web). Filters are plain HTML `<form method="GET">` elements — a full page reload on filter change. No JS framework, no HTMX, no Node build step: the MVP doesn't need partial-page updates, and a vendored or CDN-loaded JS library would add a dependency the spec never asked for.

## Global Constraints

- Data source: Bitbucket Cloud only (workspace `rdwrcloud`, project `WB`), per [2026-06-26-mvp-design.md](../specs/2026-06-26-mvp-design.md). No GitHub/GitLab in MVP.
- Entities: Repository, Commit, PullRequest, Review, Author, FileChange only.
- Dashboards: Delivery flow, Code churn, Activity (author breakdown). No Overview dashboard, no CSV export, no alerts/anomaly detection, no RBAC/audit logs — all explicitly out of scope.
- Config is a single static YAML file (repo list, author allowlist, sync interval). No admin UI for managing it.
- Bitbucket credentials come from environment variables only, never from the config file.
- Storage is SQLite, one file, no separate DB server.
- A failed sync for one repo must not block other repos and must leave that repo's watermark unchanged (retry from same point next time).
- No partial/duplicate writes on a failed sync.
- Must run locally with nothing but a Go toolchain: `go run ./cmd/server` and no other services to install.
- Do not add anything not in this list or in the explicit scope above (YAGNI) — e.g. no caching layer, no message queue, no multi-tenancy, no auth/login system for MVP.

## Definition of Done

The MVP is done when all of the following are true, verified by running the commands shown:

1. `go build ./...` succeeds with no errors.
2. `go test ./...` passes with no failures.
3. `go run ./cmd/server --config testdata/config.yaml` starts an HTTP server on `:8080` (with `BITBUCKET_USERNAME`/`BITBUCKET_APP_PASSWORD` env vars set, even to dummy values, since config validation requires them).
4. Visiting `http://localhost:8080/` in a browser shows three dashboard pages reachable via nav links: **Activity**, **Delivery Flow**, **Code Churn**.
5. Clicking "Sync now" on any dashboard triggers a sync (verified against a mocked Bitbucket server in the integration test described in Task 19; no live Bitbucket account is required to verify the app).
6. Each dashboard has working date-range, repo, and author filters submitted as a GET form that reloads the page with the filtered results.
7. Running the binary twice in a row against the same SQLite file does not duplicate rows (upserts are idempotent) — verified by the storage tests.
8. No code in the repo touches GitHub/GitLab APIs, CSV export, alerting, or RBAC.

---

## File Structure

```
git-statistics/
  go.mod
  cmd/
    server/
      main.go                 # wires config → storage → ingest → metrics → web → scheduler, starts HTTP server
  internal/
    config/
      config.go                # YAML config struct + Load() + validation
      config_test.go
    domain/
      domain.go                 # Repository, Author, Commit, FileChange, PullRequest, Review structs
    storage/
      sqlite.go                 # Open(), schema migration, Filter struct
      repository.go             # Repository/Author upsert + queries
      commits.go                # Commit/FileChange upsert + queries
      pullrequests.go            # PullRequest/Review upsert + queries
      storage_test.go
    bitbucket/
      client.go                 # HTTP client: ListCommits, ListPullRequests, ListActivity, GetDiffstat
      types.go                  # raw Bitbucket JSON response structs
      client_test.go
    normalize/
      normalize.go               # raw bitbucket types -> domain entities, allowlist tagging
      normalize_test.go
    ingest/
      ingest.go                  # SyncRepo(), SyncAll()
      ingest_test.go
    metrics/
      metrics.go                 # PR cycle time, time-to-first-review, time-in-review, churn, commits-per-author
      metrics_test.go
    scheduler/
      scheduler.go               # ticker-based periodic sync + manual trigger channel
      scheduler_test.go
    web/
      web.go                      # http.Handler, routes
      handlers.go                 # dashboard handlers (activity, delivery flow, churn, sync)
      templates/
        layout.html
        activity.html
        delivery_flow.html
        churn.html
      web_test.go
    integration/
      integration_test.go         # full path: fixture data -> sync -> normalize -> metrics -> sqlite -> dashboard HTTP response
  testdata/
    config.yaml                  # sample config used by integration test and local runs
docs/
  superpowers/
    plans/2026-06-26-mvp-implementation.md   # this file
    specs/2026-06-26-mvp-design.md            # existing design spec
```

---

### Task 1: Project scaffolding and health endpoint

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Test: `cmd/server/main_test.go`

**Interfaces:**
- Produces: a runnable binary entrypoint and an HTTP server skeleton later tasks attach handlers to.

- [ ] **Step 1: Initialize the Go module**

Run: `go mod init git-statistics`
Expected: creates `go.mod` with `module git-statistics` and a `go` directive.

- [ ] **Step 2: Write the failing test for the health endpoint**

```go
// cmd/server/main_test.go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	mux := newMux()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", rec.Body.String())
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./cmd/server/...`
Expected: FAIL — `newMux` undefined.

- [ ] **Step 4: Write minimal implementation**

```go
// cmd/server/main.go
package main

import (
	"flag"
	"log"
	"net/http"
)

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config file")
	flag.Parse()

	log.Printf("starting git-statistics server with config %s", *configPath)
	mux := newMux()
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./cmd/server/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod cmd/server/main.go cmd/server/main_test.go
git commit -m "feat: scaffold server binary with health endpoint"
```

### Task 2: Domain entities

**Files:**
- Create: `internal/domain/domain.go`

**Interfaces:**
- Produces: `Repository`, `Author`, `Commit`, `FileChange`, `PullRequest`, `Review` structs used by every later package (storage, normalize, ingest, metrics, web).

No test file for this task — these are plain data structs with no behavior to assert; they're exercised indirectly by every later task's tests. This is the one task in the plan without its own red/green cycle because there is no logic to test.

- [ ] **Step 1: Write the entity structs**

```go
// internal/domain/domain.go
package domain

import "time"

type Repository struct {
	Slug      string
	Workspace string
	SyncedAt  time.Time
}

type Author struct {
	ID          string // Bitbucket account_id
	DisplayName string
	Email       string
	Allowlisted bool
}

type Commit struct {
	Hash       string
	RepoSlug   string
	AuthorID   string
	Message    string
	AuthoredAt time.Time
}

type FileChange struct {
	CommitHash   string
	RepoSlug     string
	Path         string
	LinesAdded   int
	LinesRemoved int
	Status       string // "added", "modified", "removed", "renamed"
}

type PullRequest struct {
	ID        int
	RepoSlug  string
	Title     string
	AuthorID  string
	State     string // "OPEN", "MERGED", "DECLINED", "SUPERSEDED"
	CreatedAt time.Time
	UpdatedAt time.Time
	MergedAt  *time.Time
}

type Review struct {
	ID            string
	PullRequestID int
	RepoSlug      string
	ReviewerID    string
	Action        string // "approved", "commented"
	CreatedAt     time.Time
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/domain/...`
Expected: succeeds with no output.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/domain.go
git commit -m "feat: add domain entity structs"
```

### Task 3: Config loading and validation

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Create: `testdata/config.yaml`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `config.Config{Bitbucket config.Bitbucket{Workspace, Project string; Repos []string}; SyncIntervalMinutes int; Authors []string; BitbucketUsername, BitbucketAppPassword string}` and `config.Load(path string) (Config, error)`, which reads `BITBUCKET_USERNAME`/`BITBUCKET_APP_PASSWORD` env vars and fails fast on invalid/incomplete config. Later tasks (ingest, web, main) read `Config` fields directly.

- [ ] **Step 1: Add the YAML dependency**

Run: `go get gopkg.in/yaml.v3`
Expected: adds the dependency to `go.mod`/`go.sum`.

- [ ] **Step 2: Write the failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

const validYAML = `
bitbucket:
  workspace: rdwrcloud
  project: WB
  repos:
    - repo-one
    - repo-two
sync_interval_minutes: 30
authors:
  - alice@example.com
  - bob@example.com
`

func TestLoad_Valid(t *testing.T) {
	t.Setenv("BITBUCKET_USERNAME", "svc-account")
	t.Setenv("BITBUCKET_APP_PASSWORD", "secret")
	path := writeTempConfig(t, validYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bitbucket.Workspace != "rdwrcloud" {
		t.Errorf("expected workspace rdwrcloud, got %q", cfg.Bitbucket.Workspace)
	}
	if len(cfg.Bitbucket.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(cfg.Bitbucket.Repos))
	}
	if cfg.SyncIntervalMinutes != 30 {
		t.Errorf("expected interval 30, got %d", cfg.SyncIntervalMinutes)
	}
	if len(cfg.Authors) != 2 {
		t.Errorf("expected 2 authors, got %d", len(cfg.Authors))
	}
	if cfg.BitbucketUsername != "svc-account" || cfg.BitbucketAppPassword != "secret" {
		t.Errorf("expected credentials from env, got %q/%q", cfg.BitbucketUsername, cfg.BitbucketAppPassword)
	}
}

func TestLoad_MissingCredentials(t *testing.T) {
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_APP_PASSWORD", "")
	path := writeTempConfig(t, validYAML)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
}

func TestLoad_NoRepos(t *testing.T) {
	t.Setenv("BITBUCKET_USERNAME", "svc-account")
	t.Setenv("BITBUCKET_APP_PASSWORD", "secret")
	path := writeTempConfig(t, `
bitbucket:
  workspace: rdwrcloud
  project: WB
  repos: []
sync_interval_minutes: 30
authors:
  - alice@example.com
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for empty repo list, got nil")
	}
}

func TestLoad_NoAuthors(t *testing.T) {
	t.Setenv("BITBUCKET_USERNAME", "svc-account")
	t.Setenv("BITBUCKET_APP_PASSWORD", "secret")
	path := writeTempConfig(t, `
bitbucket:
  workspace: rdwrcloud
  project: WB
  repos:
    - repo-one
sync_interval_minutes: 30
authors: []
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for empty author list, got nil")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL — package `config` / `Load` undefined.

- [ ] **Step 4: Write minimal implementation**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Bitbucket struct {
	Workspace string   `yaml:"workspace"`
	Project   string   `yaml:"project"`
	Repos     []string `yaml:"repos"`
}

type Config struct {
	Bitbucket            Bitbucket `yaml:"bitbucket"`
	SyncIntervalMinutes  int       `yaml:"sync_interval_minutes"`
	Authors              []string  `yaml:"authors"`
	BitbucketUsername    string    `yaml:"-"`
	BitbucketAppPassword string    `yaml:"-"`
}

func Load(path string) (Config, error) {
	var cfg Config

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.BitbucketUsername = os.Getenv("BITBUCKET_USERNAME")
	cfg.BitbucketAppPassword = os.Getenv("BITBUCKET_APP_PASSWORD")

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.BitbucketUsername == "" || c.BitbucketAppPassword == "" {
		return fmt.Errorf("BITBUCKET_USERNAME and BITBUCKET_APP_PASSWORD env vars must be set")
	}
	if c.Bitbucket.Workspace == "" {
		return fmt.Errorf("bitbucket.workspace must be set")
	}
	if len(c.Bitbucket.Repos) == 0 {
		return fmt.Errorf("bitbucket.repos must contain at least one repo slug")
	}
	if c.SyncIntervalMinutes <= 0 {
		return fmt.Errorf("sync_interval_minutes must be greater than 0")
	}
	if len(c.Authors) == 0 {
		return fmt.Errorf("authors must contain at least one allowlisted author")
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS

- [ ] **Step 6: Create the shared sample config used by later integration tests and local runs**

```yaml
# testdata/config.yaml
bitbucket:
  workspace: rdwrcloud
  project: WB
  repos:
    - sample-repo
sync_interval_minutes: 30
authors:
  - alice@example.com
  - bob@example.com
```

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go testdata/config.yaml go.mod go.sum
git commit -m "feat: add YAML config loading and validation"
```

### Task 4: SQLite schema and connection setup

**Files:**
- Create: `internal/storage/sqlite.go`
- Test: `internal/storage/sqlite_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `storage.Open(path string) (*Store, error)` (runs schema migration, returns a `*Store` wrapping `*sql.DB`), `storage.Filter{RepoSlug, AuthorID string; From, To time.Time}` used by every query method in Tasks 5-7, and `(*Store) Close() error`. Later tasks add methods on `*Store`.

- [ ] **Step 1: Add the SQLite driver dependency**

Run: `go get modernc.org/sqlite`
Expected: adds the pure-Go SQLite driver (no cgo needed) to `go.mod`/`go.sum`.

- [ ] **Step 2: Write the failing test**

```go
// internal/storage/sqlite_test.go
package storage

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpen_CreatesSchema(t *testing.T) {
	store := openTestStore(t)

	tables := []string{"repositories", "authors", "commits", "file_changes", "pull_requests", "reviews"}
	for _, table := range tables {
		var name string
		row := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		if err := row.Scan(&name); err != nil {
			t.Errorf("expected table %q to exist: %v", table, err)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/storage/...`
Expected: FAIL - `Open` undefined.

- [ ] **Step 4: Write minimal implementation**

```go
// internal/storage/sqlite.go
package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Filter struct {
	RepoSlug string
	AuthorID string
	From     time.Time
	To       time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS repositories (
	slug TEXT PRIMARY KEY,
	workspace TEXT NOT NULL,
	synced_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS authors (
	id TEXT PRIMARY KEY,
	display_name TEXT,
	email TEXT,
	allowlisted INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS commits (
	hash TEXT NOT NULL,
	repo_slug TEXT NOT NULL,
	author_id TEXT,
	message TEXT,
	authored_at TIMESTAMP NOT NULL,
	PRIMARY KEY (hash, repo_slug)
);

CREATE TABLE IF NOT EXISTS file_changes (
	commit_hash TEXT NOT NULL,
	repo_slug TEXT NOT NULL,
	path TEXT NOT NULL,
	lines_added INTEGER NOT NULL DEFAULT 0,
	lines_removed INTEGER NOT NULL DEFAULT 0,
	status TEXT,
	PRIMARY KEY (commit_hash, repo_slug, path)
);

CREATE TABLE IF NOT EXISTS pull_requests (
	id INTEGER NOT NULL,
	repo_slug TEXT NOT NULL,
	title TEXT,
	author_id TEXT,
	state TEXT,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP,
	merged_at TIMESTAMP,
	PRIMARY KEY (id, repo_slug)
);

CREATE TABLE IF NOT EXISTS reviews (
	id TEXT PRIMARY KEY,
	pull_request_id INTEGER NOT NULL,
	repo_slug TEXT NOT NULL,
	reviewer_id TEXT,
	action TEXT,
	created_at TIMESTAMP NOT NULL
);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("running schema migration: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/storage/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage/sqlite.go internal/storage/sqlite_test.go go.mod go.sum
git commit -m "feat: add sqlite schema and connection setup"
```
### Task 5: Storage - Repository and Author upsert/query

**Files:**
- Create: `internal/storage/repository.go`
- Test: `internal/storage/repository_test.go`

**Interfaces:**
- Consumes: `Store`, `Filter` from Task 4; `domain.Repository`, `domain.Author` from Task 2.
- Produces: `(*Store) UpsertRepository(domain.Repository) error`, `(*Store) GetRepositorySyncedAt(slug string) (time.Time, error)` (returns zero `time.Time` if repo not yet synced), `(*Store) UpsertAuthor(domain.Author) error`, `(*Store) ListAuthors() ([]domain.Author, error)`. Used by `ingest` (Task 12) and `web` (Task 13).

- [ ] **Step 1: Write the failing tests**

```go
// internal/storage/repository_test.go
package storage

import (
	"testing"
	"time"

	"git-statistics/internal/domain"
)

func TestUpsertRepository_AndGetSyncedAt(t *testing.T) {
	store := openTestStore(t)

	zero, err := store.GetRepositorySyncedAt("repo-one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !zero.IsZero() {
		t.Fatalf("expected zero time for unsynced repo, got %v", zero)
	}

	syncedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	err = store.UpsertRepository(domain.Repository{Slug: "repo-one", Workspace: "rdwrcloud", SyncedAt: syncedAt})
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	got, err := store.GetRepositorySyncedAt("repo-one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(syncedAt) {
		t.Errorf("expected synced_at %v, got %v", syncedAt, got)
	}

	later := syncedAt.Add(time.Hour)
	if err := store.UpsertRepository(domain.Repository{Slug: "repo-one", Workspace: "rdwrcloud", SyncedAt: later}); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	got, _ = store.GetRepositorySyncedAt("repo-one")
	if !got.Equal(later) {
		t.Errorf("expected synced_at %v after second upsert, got %v", later, got)
	}

	var count int
	row := store.db.QueryRow("SELECT COUNT(*) FROM repositories WHERE slug = ?", "repo-one")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for repo-one, got %d", count)
	}
}

func TestUpsertAuthor_AndList(t *testing.T) {
	store := openTestStore(t)

	a := domain.Author{ID: "acct-1", DisplayName: "Alice", Email: "alice@example.com", Allowlisted: true}
	if err := store.UpsertAuthor(a); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	a.Allowlisted = false
	if err := store.UpsertAuthor(a); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	authors, err := store.ListAuthors()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("expected 1 author, got %d", len(authors))
	}
	if authors[0].Allowlisted {
		t.Errorf("expected allowlisted=false after update, got true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/...`
Expected: FAIL — `UpsertRepository` etc. undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/storage/repository.go
package storage

import (
	"database/sql"
	"time"

	"git-statistics/internal/domain"
)

func (s *Store) UpsertRepository(r domain.Repository) error {
	_, err := s.db.Exec(`
		INSERT INTO repositories (slug, workspace, synced_at)
		VALUES (?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET workspace = excluded.workspace, synced_at = excluded.synced_at
	`, r.Slug, r.Workspace, r.SyncedAt)
	return err
}

func (s *Store) GetRepositorySyncedAt(slug string) (time.Time, error) {
	var syncedAt sql.NullTime
	row := s.db.QueryRow("SELECT synced_at FROM repositories WHERE slug = ?", slug)
	err := row.Scan(&syncedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	if !syncedAt.Valid {
		return time.Time{}, nil
	}
	return syncedAt.Time, nil
}

func (s *Store) UpsertAuthor(a domain.Author) error {
	_, err := s.db.Exec(`
		INSERT INTO authors (id, display_name, email, allowlisted)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET display_name = excluded.display_name, email = excluded.email, allowlisted = excluded.allowlisted
	`, a.ID, a.DisplayName, a.Email, a.Allowlisted)
	return err
}

func (s *Store) ListAuthors() ([]domain.Author, error) {
	rows, err := s.db.Query("SELECT id, display_name, email, allowlisted FROM authors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []domain.Author
	for rows.Next() {
		var a domain.Author
		if err := rows.Scan(&a.ID, &a.DisplayName, &a.Email, &a.Allowlisted); err != nil {
			return nil, err
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/repository.go internal/storage/repository_test.go
git commit -m "feat: add repository and author storage"
```
### Task 6: Storage - Commit and FileChange upsert/query

**Files:**
- Create: `internal/storage/commits.go`
- Test: `internal/storage/commits_test.go`

**Interfaces:**
- Consumes: `Store`, `Filter` from Task 4; `domain.Commit`, `domain.FileChange` from Task 2.
- Produces: `(*Store) UpsertCommit(domain.Commit) error`, `(*Store) UpsertFileChange(domain.FileChange) error`, `(*Store) ListCommits(Filter) ([]domain.Commit, error)`, `(*Store) ListFileChanges(Filter) ([]domain.FileChange, error)`. `Filter.RepoSlug`/`AuthorID` empty string means "no filter on that field"; `Filter.From`/`To` zero value means "no bound on that side". Used by `metrics` (Task 13) and `ingest` (Task 12).

- [ ] **Step 1: Write the failing tests**

```go
// internal/storage/commits_test.go
package storage

import (
	"testing"
	"time"

	"git-statistics/internal/domain"
)

func TestUpsertCommit_IdempotentAndFilterable(t *testing.T) {
	store := openTestStore(t)

	c := domain.Commit{
		Hash: "abc123", RepoSlug: "repo-one", AuthorID: "acct-1",
		Message: "fix bug", AuthoredAt: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
	}
	if err := store.UpsertCommit(c); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := store.UpsertCommit(c); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	other := domain.Commit{
		Hash: "def456", RepoSlug: "repo-two", AuthorID: "acct-2",
		Message: "add feature", AuthoredAt: time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
	}
	if err := store.UpsertCommit(other); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	all, err := store.ListCommits(Filter{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 commits total, got %d", len(all))
	}

	filtered, err := store.ListCommits(Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Hash != "abc123" {
		t.Fatalf("expected only abc123 for repo-one, got %+v", filtered)
	}

	byDate, err := store.ListCommits(Filter{From: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(byDate) != 1 || byDate[0].Hash != "def456" {
		t.Fatalf("expected only def456 after 2026-01-20, got %+v", byDate)
	}
}

func TestUpsertFileChange_IdempotentAndFilterable(t *testing.T) {
	store := openTestStore(t)

	fc := domain.FileChange{
		CommitHash: "abc123", RepoSlug: "repo-one", Path: "main.go",
		LinesAdded: 10, LinesRemoved: 2, Status: "modified",
	}
	if err := store.UpsertFileChange(fc); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	fc.LinesAdded = 15
	if err := store.UpsertFileChange(fc); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	changes, err := store.ListFileChanges(Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 file change, got %d", len(changes))
	}
	if changes[0].LinesAdded != 15 {
		t.Errorf("expected updated LinesAdded 15, got %d", changes[0].LinesAdded)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/...`
Expected: FAIL — `UpsertCommit` etc. undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/storage/commits.go
package storage

import (
	"strings"

	"git-statistics/internal/domain"
)

func (s *Store) UpsertCommit(c domain.Commit) error {
	_, err := s.db.Exec(`
		INSERT INTO commits (hash, repo_slug, author_id, message, authored_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(hash, repo_slug) DO UPDATE SET
			author_id = excluded.author_id, message = excluded.message, authored_at = excluded.authored_at
	`, c.Hash, c.RepoSlug, c.AuthorID, c.Message, c.AuthoredAt)
	return err
}

func (s *Store) UpsertFileChange(fc domain.FileChange) error {
	_, err := s.db.Exec(`
		INSERT INTO file_changes (commit_hash, repo_slug, path, lines_added, lines_removed, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(commit_hash, repo_slug, path) DO UPDATE SET
			lines_added = excluded.lines_added, lines_removed = excluded.lines_removed, status = excluded.status
	`, fc.CommitHash, fc.RepoSlug, fc.Path, fc.LinesAdded, fc.LinesRemoved, fc.Status)
	return err
}

func buildCommitFilter(f Filter) (string, []any) {
	var clauses []string
	var args []any
	if f.RepoSlug != "" {
		clauses = append(clauses, "repo_slug = ?")
		args = append(args, f.RepoSlug)
	}
	if f.AuthorID != "" {
		clauses = append(clauses, "author_id = ?")
		args = append(args, f.AuthorID)
	}
	if !f.From.IsZero() {
		clauses = append(clauses, "authored_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		clauses = append(clauses, "authored_at <= ?")
		args = append(args, f.To)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *Store) ListCommits(f Filter) ([]domain.Commit, error) {
	where, args := buildCommitFilter(f)
	rows, err := s.db.Query("SELECT hash, repo_slug, author_id, message, authored_at FROM commits"+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commits []domain.Commit
	for rows.Next() {
		var c domain.Commit
		if err := rows.Scan(&c.Hash, &c.RepoSlug, &c.AuthorID, &c.Message, &c.AuthoredAt); err != nil {
			return nil, err
		}
		commits = append(commits, c)
	}
	return commits, rows.Err()
}

func (s *Store) ListFileChanges(f Filter) ([]domain.FileChange, error) {
	var clauses []string
	var args []any
	if f.RepoSlug != "" {
		clauses = append(clauses, "repo_slug = ?")
		args = append(args, f.RepoSlug)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	query := "SELECT fc.commit_hash, fc.repo_slug, fc.path, fc.lines_added, fc.lines_removed, fc.status FROM file_changes fc"
	if f.RepoSlug != "" {
		query += where
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []domain.FileChange
	for rows.Next() {
		var fc domain.FileChange
		if err := rows.Scan(&fc.CommitHash, &fc.RepoSlug, &fc.Path, &fc.LinesAdded, &fc.LinesRemoved, &fc.Status); err != nil {
			return nil, err
		}
		changes = append(changes, fc)
	}
	return changes, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/commits.go internal/storage/commits_test.go
git commit -m "feat: add commit and file change storage"
```
### Task 7: Storage - PullRequest and Review upsert/query

**Files:**
- Create: `internal/storage/pullrequests.go`
- Test: `internal/storage/pullrequests_test.go`

**Interfaces:**
- Consumes: `Store`, `Filter` from Task 4; `domain.PullRequest`, `domain.Review` from Task 2.
- Produces: `(*Store) UpsertPullRequest(domain.PullRequest) error`, `(*Store) UpsertReview(domain.Review) error`, `(*Store) ListPullRequests(Filter) ([]domain.PullRequest, error)`, `(*Store) ListReviews(repoSlug string, pullRequestID int) ([]domain.Review, error)`. Used by `metrics` (Task 13) and `ingest` (Task 12).

- [ ] **Step 1: Write the failing tests**

```go
// internal/storage/pullrequests_test.go
package storage

import (
	"testing"
	"time"

	"git-statistics/internal/domain"
)

func TestUpsertPullRequest_IdempotentAndFilterable(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	merged := created.Add(48 * time.Hour)
	pr := domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}
	if err := store.UpsertPullRequest(pr); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	pr.State = "MERGED"
	if err := store.UpsertPullRequest(pr); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	prs, err := store.ListPullRequests(Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].MergedAt == nil || !prs[0].MergedAt.Equal(merged) {
		t.Errorf("expected MergedAt %v, got %v", merged, prs[0].MergedAt)
	}
}

func TestUpsertReview_AndListByPullRequest(t *testing.T) {
	store := openTestStore(t)

	r := domain.Review{
		ID: "rev-1", PullRequestID: 1, RepoSlug: "repo-one",
		ReviewerID: "acct-2", Action: "approved",
		CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := store.UpsertReview(r); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := store.UpsertReview(r); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	reviews, err := store.ListReviews("repo-one", 1)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(reviews))
	}

	none, err := store.ListReviews("repo-one", 2)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected 0 reviews for PR 2, got %d", len(none))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/...`
Expected: FAIL — `UpsertPullRequest` etc. undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/storage/pullrequests.go
package storage

import (
	"database/sql"
	"strings"

	"git-statistics/internal/domain"
)

func (s *Store) UpsertPullRequest(pr domain.PullRequest) error {
	var mergedAt sql.NullTime
	if pr.MergedAt != nil {
		mergedAt = sql.NullTime{Time: *pr.MergedAt, Valid: true}
	}
	_, err := s.db.Exec(`
		INSERT INTO pull_requests (id, repo_slug, title, author_id, state, created_at, updated_at, merged_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, repo_slug) DO UPDATE SET
			title = excluded.title, author_id = excluded.author_id, state = excluded.state,
			updated_at = excluded.updated_at, merged_at = excluded.merged_at
	`, pr.ID, pr.RepoSlug, pr.Title, pr.AuthorID, pr.State, pr.CreatedAt, pr.UpdatedAt, mergedAt)
	return err
}

func (s *Store) UpsertReview(r domain.Review) error {
	_, err := s.db.Exec(`
		INSERT INTO reviews (id, pull_request_id, repo_slug, reviewer_id, action, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			reviewer_id = excluded.reviewer_id, action = excluded.action, created_at = excluded.created_at
	`, r.ID, r.PullRequestID, r.RepoSlug, r.ReviewerID, r.Action, r.CreatedAt)
	return err
}

func (s *Store) ListPullRequests(f Filter) ([]domain.PullRequest, error) {
	var clauses []string
	var args []any
	if f.RepoSlug != "" {
		clauses = append(clauses, "repo_slug = ?")
		args = append(args, f.RepoSlug)
	}
	if f.AuthorID != "" {
		clauses = append(clauses, "author_id = ?")
		args = append(args, f.AuthorID)
	}
	if !f.From.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, f.To)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := s.db.Query(`
		SELECT id, repo_slug, title, author_id, state, created_at, updated_at, merged_at
		FROM pull_requests`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []domain.PullRequest
	for rows.Next() {
		var pr domain.PullRequest
		var mergedAt sql.NullTime
		if err := rows.Scan(&pr.ID, &pr.RepoSlug, &pr.Title, &pr.AuthorID, &pr.State, &pr.CreatedAt, &pr.UpdatedAt, &mergedAt); err != nil {
			return nil, err
		}
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}
		prs = append(prs, pr)
	}
	return prs, rows.Err()
}

func (s *Store) ListReviews(repoSlug string, pullRequestID int) ([]domain.Review, error) {
	rows, err := s.db.Query(`
		SELECT id, pull_request_id, repo_slug, reviewer_id, action, created_at
		FROM reviews WHERE repo_slug = ? AND pull_request_id = ?
		ORDER BY created_at ASC
	`, repoSlug, pullRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var r domain.Review
		if err := rows.Scan(&r.ID, &r.PullRequestID, &r.RepoSlug, &r.ReviewerID, &r.Action, &r.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/pullrequests.go internal/storage/pullrequests_test.go
git commit -m "feat: add pull request and review storage"
```
### Task 8: Bitbucket client - commits and diffstat

**Files:**
- Create: `internal/bitbucket/types.go`
- Create: `internal/bitbucket/client.go`
- Test: `internal/bitbucket/client_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks (standalone HTTP client).
- Produces: `bitbucket.Client{BaseURL, Username, AppPassword string; HTTPClient *http.Client}`, `bitbucket.NewClient(username, appPassword string) *Client` (defaults `BaseURL` to `https://api.bitbucket.org/2.0`, `HTTPClient` to `http.DefaultClient`), `(*Client) ListCommits(ctx, workspace, repoSlug string, since time.Time) ([]bitbucket.RawCommit, error)`, `(*Client) GetDiffstat(ctx, workspace, repoSlug, commitHash string) ([]bitbucket.RawDiffstatEntry, error)`. Raw types and these methods are consumed by `normalize` (Task 10) and `ingest` (Task 12).

- [ ] **Step 1: Write the raw response types**

```go
// internal/bitbucket/types.go
package bitbucket

type RawAuthor struct {
	Raw  string `json:"raw"`
	User struct {
		AccountID   string `json:"account_id"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
}

type RawCommit struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Date    string    `json:"date"`
	Author  RawAuthor `json:"author"`
}

type commitsPage struct {
	Values []RawCommit `json:"values"`
	Next   string      `json:"next"`
}

type RawDiffstatEntry struct {
	Status       string `json:"status"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	New          *struct {
		Path string `json:"path"`
	} `json:"new"`
	Old *struct {
		Path string `json:"path"`
	} `json:"old"`
}

type diffstatPage struct {
	Values []RawDiffstatEntry `json:"values"`
	Next   string             `json:"next"`
}

type RawPullRequest struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	CreatedOn string    `json:"created_on"`
	UpdatedOn string    `json:"updated_on"`
	Author    RawAuthor `json:"author"`
}

type pullRequestsPage struct {
	Values []RawPullRequest `json:"values"`
	Next   string           `json:"next"`
}

type RawActivity struct {
	Approval *struct {
		Date string    `json:"date"`
		User RawAuthor `json:"user"`
	} `json:"approval"`
	Comment *struct {
		CreatedOn string    `json:"created_on"`
		User      RawAuthor `json:"user"`
	} `json:"comment"`
}

type activityPage struct {
	Values []RawActivity `json:"values"`
	Next   string        `json:"next"`
}
```

- [ ] **Step 2: Write the failing test for ListCommits and GetDiffstat**

```go
// internal/bitbucket/client_test.go
package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListCommits_PaginatesAndParses(t *testing.T) {
	var pageTwoURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "svc" || pass != "secret" {
			t.Errorf("expected basic auth svc/secret, got %s/%s (ok=%v)", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/repositories/rdwrcloud/repo-one/commits" {
			_, _ = w.Write([]byte(`{
				"values": [{"hash":"abc","message":"first","date":"2026-01-01T10:00:00Z","author":{"raw":"Alice <alice@example.com>","user":{"account_id":"acct-1","display_name":"Alice"}}}],
				"next": "` + pageTwoURL + `"
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"values": [{"hash":"def","message":"second","date":"2026-01-02T10:00:00Z","author":{"raw":"Bob <bob@example.com>","user":{"account_id":"acct-2","display_name":"Bob"}}}],
			"next": ""
		}`))
	}))
	defer server.Close()
	pageTwoURL = server.URL + "/page2"

	client := NewClient("svc", "secret")
	client.BaseURL = server.URL

	commits, err := client.ListCommits(context.Background(), "rdwrcloud", "repo-one", time.Time{})
	if err != nil {
		t.Fatalf("ListCommits failed: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits across pages, got %d", len(commits))
	}
	if commits[0].Hash != "abc" || commits[1].Hash != "def" {
		t.Errorf("unexpected commit order/content: %+v", commits)
	}
}

func TestGetDiffstat_ParsesEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"values": [{"status":"modified","lines_added":5,"lines_removed":1,"new":{"path":"main.go"},"old":{"path":"main.go"}}],
			"next": ""
		}`))
	}))
	defer server.Close()

	client := NewClient("svc", "secret")
	client.BaseURL = server.URL

	entries, err := client.GetDiffstat(context.Background(), "rdwrcloud", "repo-one", "abc")
	if err != nil {
		t.Fatalf("GetDiffstat failed: %v", err)
	}
	if len(entries) != 1 || entries[0].New.Path != "main.go" {
		t.Fatalf("unexpected diffstat entries: %+v", entries)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/bitbucket/...`
Expected: FAIL — `NewClient` undefined.

- [ ] **Step 4: Write minimal implementation**

```go
// internal/bitbucket/client.go
package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL     string
	Username    string
	AppPassword string
	HTTPClient  *http.Client
}

func NewClient(username, appPassword string) *Client {
	return &Client{
		BaseURL:     "https://api.bitbucket.org/2.0",
		Username:    username,
		AppPassword: appPassword,
		HTTPClient:  http.DefaultClient,
	}
}

func (c *Client) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.AppPassword)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bitbucket API request to %s failed: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) ListCommits(ctx context.Context, workspace, repoSlug string, since time.Time) ([]RawCommit, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/commits", c.BaseURL, workspace, repoSlug)

	var all []RawCommit
	for url != "" {
		var page commitsPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing commits for %s/%s: %w", workspace, repoSlug, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}

func (c *Client) GetDiffstat(ctx context.Context, workspace, repoSlug, commitHash string) ([]RawDiffstatEntry, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/diffstat/%s", c.BaseURL, workspace, repoSlug, commitHash)

	var all []RawDiffstatEntry
	for url != "" {
		var page diffstatPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("getting diffstat for %s/%s@%s: %w", workspace, repoSlug, commitHash, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}
```

Note: `since` is accepted now for the Task 12 watermark-based incremental sync but is not used to filter the API call directly (Bitbucket's commits endpoint has no date filter query param). Incremental behavior is implemented in `ingest` (Task 12) by skipping commits older than the watermark once fetched — this task is pagination and parsing only.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/bitbucket/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/bitbucket/types.go internal/bitbucket/client.go internal/bitbucket/client_test.go
git commit -m "feat: add bitbucket client for commits and diffstat"
```
### Task 9: Bitbucket client - pull requests and review activity

**Files:**
- Modify: `internal/bitbucket/client.go`
- Modify: `internal/bitbucket/client_test.go`

**Interfaces:**
- Consumes: `Client` from Task 8.
- Produces: `(*Client) ListPullRequests(ctx, workspace, repoSlug string) ([]RawPullRequest, error)`, `(*Client) ListActivity(ctx, workspace, repoSlug string, pullRequestID int) ([]RawActivity, error)`. Consumed by `normalize` (Task 11) and `ingest` (Task 12).

- [ ] **Step 1: Write the failing tests**

```go
// append to internal/bitbucket/client_test.go

func TestListPullRequests_ParsesAllStates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"values": [{"id":1,"title":"Add feature","state":"MERGED","created_on":"2026-01-01T09:00:00Z","updated_on":"2026-01-03T09:00:00Z","author":{"raw":"Alice <alice@example.com>","user":{"account_id":"acct-1","display_name":"Alice"}}}],
			"next": ""
		}`))
	}))
	defer server.Close()

	client := NewClient("svc", "secret")
	client.BaseURL = server.URL

	prs, err := client.ListPullRequests(context.Background(), "rdwrcloud", "repo-one")
	if err != nil {
		t.Fatalf("ListPullRequests failed: %v", err)
	}
	if len(prs) != 1 || prs[0].ID != 1 {
		t.Fatalf("unexpected pull requests: %+v", prs)
	}
}

func TestListActivity_ParsesApprovalsAndComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"values": [
				{"approval":{"date":"2026-01-02T09:00:00Z","user":{"raw":"Bob <bob@example.com>","user":{"account_id":"acct-2","display_name":"Bob"}}}},
				{"comment":{"created_on":"2026-01-02T08:00:00Z","user":{"raw":"Bob <bob@example.com>","user":{"account_id":"acct-2","display_name":"Bob"}}}}
			],
			"next": ""
		}`))
	}))
	defer server.Close()

	client := NewClient("svc", "secret")
	client.BaseURL = server.URL

	activity, err := client.ListActivity(context.Background(), "rdwrcloud", "repo-one", 1)
	if err != nil {
		t.Fatalf("ListActivity failed: %v", err)
	}
	if len(activity) != 2 {
		t.Fatalf("expected 2 activity entries, got %d", len(activity))
	}
	if activity[0].Approval == nil {
		t.Errorf("expected first entry to be an approval")
	}
	if activity[1].Comment == nil {
		t.Errorf("expected second entry to be a comment")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bitbucket/...`
Expected: FAIL — `ListPullRequests` / `ListActivity` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/bitbucket/client.go

func (c *Client) ListPullRequests(ctx context.Context, workspace, repoSlug string) ([]RawPullRequest, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests?state=MERGED&state=OPEN&state=DECLINED&state=SUPERSEDED", c.BaseURL, workspace, repoSlug)

	var all []RawPullRequest
	for url != "" {
		var page pullRequestsPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing pull requests for %s/%s: %w", workspace, repoSlug, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}

func (c *Client) ListActivity(ctx context.Context, workspace, repoSlug string, pullRequestID int) ([]RawActivity, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/activity", c.BaseURL, workspace, repoSlug, pullRequestID)

	var all []RawActivity
	for url != "" {
		var page activityPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing activity for %s/%s PR %d: %w", workspace, repoSlug, pullRequestID, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bitbucket/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bitbucket/client.go internal/bitbucket/client_test.go
git commit -m "feat: add bitbucket client for pull requests and review activity"
```
### Task 10: Normalize - commits and file changes

**Files:**
- Create: `internal/normalize/normalize.go`
- Test: `internal/normalize/normalize_test.go`

**Interfaces:**
- Consumes: `bitbucket.RawCommit`, `bitbucket.RawDiffstatEntry` from Tasks 8-9; `domain.Commit`, `domain.FileChange`, `domain.Author` from Task 2.
- Produces: `normalize.Commit(repoSlug string, raw bitbucket.RawCommit) (domain.Commit, domain.Author, error)` (parses the RFC3339 date, extracts email from the `raw` field's `Name <email>` format), `normalize.FileChanges(repoSlug, commitHash string, raw []bitbucket.RawDiffstatEntry) []domain.FileChange`, `normalize.IsAllowlisted(author domain.Author, allowlist []string) bool` (matches on email, case-insensitive). Consumed by `ingest` (Task 12).

- [ ] **Step 1: Write the failing tests**

```go
// internal/normalize/normalize_test.go
package normalize

import (
	"testing"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/domain"
)

func TestCommit_ParsesFieldsAndAuthor(t *testing.T) {
	raw := bitbucket.RawCommit{
		Hash:    "abc123",
		Message: "fix bug",
		Date:    "2026-01-05T10:30:00Z",
	}
	raw.Author.Raw = "Alice Smith <alice@example.com>"
	raw.Author.User.AccountID = "acct-1"
	raw.Author.User.DisplayName = "Alice Smith"

	commit, author, err := Commit("repo-one", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commit.Hash != "abc123" || commit.RepoSlug != "repo-one" || commit.AuthorID != "acct-1" {
		t.Errorf("unexpected commit: %+v", commit)
	}
	want := time.Date(2026, 1, 5, 10, 30, 0, 0, time.UTC)
	if !commit.AuthoredAt.Equal(want) {
		t.Errorf("expected AuthoredAt %v, got %v", want, commit.AuthoredAt)
	}
	if author.Email != "alice@example.com" || author.DisplayName != "Alice Smith" {
		t.Errorf("unexpected author: %+v", author)
	}
}

func TestCommit_InvalidDateReturnsError(t *testing.T) {
	raw := bitbucket.RawCommit{Hash: "abc", Date: "not-a-date"}
	if _, _, err := Commit("repo-one", raw); err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
}

func TestFileChanges_MapsAddedModifiedRemoved(t *testing.T) {
	raw := []bitbucket.RawDiffstatEntry{
		{Status: "modified", LinesAdded: 5, LinesRemoved: 1, New: &struct{ Path string `json:"path"` }{Path: "main.go"}},
		{Status: "removed", LinesAdded: 0, LinesRemoved: 20, Old: &struct{ Path string `json:"path"` }{Path: "old.go"}},
	}

	changes := FileChanges("repo-one", "abc123", raw)

	if len(changes) != 2 {
		t.Fatalf("expected 2 file changes, got %d", len(changes))
	}
	if changes[0].Path != "main.go" || changes[0].LinesAdded != 5 {
		t.Errorf("unexpected first file change: %+v", changes[0])
	}
	if changes[1].Path != "old.go" || changes[1].LinesRemoved != 20 {
		t.Errorf("unexpected second file change: %+v", changes[1])
	}
}

func TestIsAllowlisted_CaseInsensitiveEmailMatch(t *testing.T) {
	author := domain.Author{ID: "acct-x", Email: "Alice@Example.com"}
	allowlist := []string{"alice@example.com", "bob@example.com"}

	if !IsAllowlisted(author, allowlist) {
		t.Error("expected case-insensitive match to succeed")
	}

	other := domain.Author{ID: "acct-y", Email: "carol@example.com"}
	if IsAllowlisted(other, allowlist) {
		t.Error("expected carol@example.com to not be allowlisted")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/normalize/...`
Expected: FAIL — `Commit`, `FileChanges`, `IsAllowlisted` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/normalize/normalize.go
package normalize

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/domain"
)

var emailPattern = regexp.MustCompile(`<([^>]+)>`)

func Commit(repoSlug string, raw bitbucket.RawCommit) (domain.Commit, domain.Author, error) {
	authoredAt, err := time.Parse(time.RFC3339, raw.Date)
	if err != nil {
		return domain.Commit{}, domain.Author{}, fmt.Errorf("parsing commit date %q: %w", raw.Date, err)
	}

	author := domain.Author{
		ID:          raw.Author.User.AccountID,
		DisplayName: raw.Author.User.DisplayName,
		Email:       extractEmail(raw.Author.Raw),
	}

	commit := domain.Commit{
		Hash:       raw.Hash,
		RepoSlug:   repoSlug,
		AuthorID:   author.ID,
		Message:    raw.Message,
		AuthoredAt: authoredAt,
	}
	return commit, author, nil
}

func extractEmail(raw string) string {
	match := emailPattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func FileChanges(repoSlug, commitHash string, raw []bitbucket.RawDiffstatEntry) []domain.FileChange {
	changes := make([]domain.FileChange, 0, len(raw))
	for _, entry := range raw {
		path := ""
		if entry.New != nil {
			path = entry.New.Path
		} else if entry.Old != nil {
			path = entry.Old.Path
		}
		changes = append(changes, domain.FileChange{
			CommitHash:   commitHash,
			RepoSlug:     repoSlug,
			Path:         path,
			LinesAdded:   entry.LinesAdded,
			LinesRemoved: entry.LinesRemoved,
			Status:       entry.Status,
		})
	}
	return changes
}

func IsAllowlisted(author domain.Author, allowlist []string) bool {
	for _, email := range allowlist {
		if strings.EqualFold(email, author.Email) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/normalize/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/normalize/normalize.go internal/normalize/normalize_test.go
git commit -m "feat: add normalize layer for commits and file changes"
```
### Task 11: Normalize - pull requests and reviews

**Files:**
- Modify: `internal/normalize/normalize.go`
- Modify: `internal/normalize/normalize_test.go`

**Interfaces:**
- Consumes: `bitbucket.RawPullRequest`, `bitbucket.RawActivity` from Task 9.
- Produces: `normalize.PullRequest(repoSlug string, raw bitbucket.RawPullRequest) (domain.PullRequest, domain.Author, error)` (sets `MergedAt` to `&UpdatedAt` when `State == "MERGED"`, else `nil`), `normalize.Review(repoSlug string, pullRequestID int, raw bitbucket.RawActivity) (domain.Review, domain.Author, bool)` (the `bool` is false if the activity entry is neither an approval nor a comment, and should be skipped). Consumed by `ingest` (Task 12).

- [ ] **Step 1: Write the failing tests**

```go
// append to internal/normalize/normalize_test.go

func TestPullRequest_MergedSetsMergedAt(t *testing.T) {
	raw := bitbucket.RawPullRequest{
		ID: 1, Title: "Add feature", State: "MERGED",
		CreatedOn: "2026-01-01T09:00:00Z", UpdatedOn: "2026-01-03T09:00:00Z",
	}
	raw.Author.Raw = "Alice <alice@example.com>"
	raw.Author.User.AccountID = "acct-1"

	pr, author, err := PullRequest("repo-one", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.MergedAt == nil {
		t.Fatal("expected MergedAt to be set for a MERGED PR")
	}
	if !pr.MergedAt.Equal(pr.UpdatedAt) {
		t.Errorf("expected MergedAt to equal UpdatedAt, got %v vs %v", pr.MergedAt, pr.UpdatedAt)
	}
	if author.ID != "acct-1" {
		t.Errorf("unexpected author: %+v", author)
	}
}

func TestPullRequest_OpenLeavesMergedAtNil(t *testing.T) {
	raw := bitbucket.RawPullRequest{
		ID: 2, Title: "WIP", State: "OPEN",
		CreatedOn: "2026-01-01T09:00:00Z", UpdatedOn: "2026-01-01T09:00:00Z",
	}
	pr, _, err := PullRequest("repo-one", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.MergedAt != nil {
		t.Errorf("expected MergedAt nil for an OPEN PR, got %v", pr.MergedAt)
	}
}

func TestReview_ApprovalMapsToActionApproved(t *testing.T) {
	raw := bitbucket.RawActivity{}
	raw.Approval = &struct {
		Date string              `json:"date"`
		User bitbucket.RawAuthor `json:"user"`
	}{Date: "2026-01-02T09:00:00Z"}
	raw.Approval.User.Raw = "Bob <bob@example.com>"
	raw.Approval.User.User.AccountID = "acct-2"

	review, author, ok := Review("repo-one", 1, raw)
	if !ok {
		t.Fatal("expected ok=true for an approval activity entry")
	}
	if review.Action != "approved" || review.ReviewerID != "acct-2" {
		t.Errorf("unexpected review: %+v", review)
	}
	if author.ID != "acct-2" {
		t.Errorf("unexpected author: %+v", author)
	}
}

func TestReview_UnrecognizedActivitySkipped(t *testing.T) {
	raw := bitbucket.RawActivity{}
	_, _, ok := Review("repo-one", 1, raw)
	if ok {
		t.Error("expected ok=false for an activity entry with no approval or comment")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/normalize/...`
Expected: FAIL — `PullRequest`, `Review` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/normalize/normalize.go

func PullRequest(repoSlug string, raw bitbucket.RawPullRequest) (domain.PullRequest, domain.Author, error) {
	createdAt, err := time.Parse(time.RFC3339, raw.CreatedOn)
	if err != nil {
		return domain.PullRequest{}, domain.Author{}, fmt.Errorf("parsing PR created_on %q: %w", raw.CreatedOn, err)
	}
	updatedAt, err := time.Parse(time.RFC3339, raw.UpdatedOn)
	if err != nil {
		return domain.PullRequest{}, domain.Author{}, fmt.Errorf("parsing PR updated_on %q: %w", raw.UpdatedOn, err)
	}

	author := domain.Author{
		ID:          raw.Author.User.AccountID,
		DisplayName: raw.Author.User.DisplayName,
		Email:       extractEmail(raw.Author.Raw),
	}

	pr := domain.PullRequest{
		ID:        raw.ID,
		RepoSlug:  repoSlug,
		Title:     raw.Title,
		AuthorID:  author.ID,
		State:     raw.State,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if raw.State == "MERGED" {
		mergedAt := updatedAt
		pr.MergedAt = &mergedAt
	}
	return pr, author, nil
}

func Review(repoSlug string, pullRequestID int, raw bitbucket.RawActivity) (domain.Review, domain.Author, bool) {
	switch {
	case raw.Approval != nil:
		createdAt, err := time.Parse(time.RFC3339, raw.Approval.Date)
		if err != nil {
			return domain.Review{}, domain.Author{}, false
		}
		author := domain.Author{
			ID:          raw.Approval.User.User.AccountID,
			DisplayName: raw.Approval.User.User.DisplayName,
			Email:       extractEmail(raw.Approval.User.Raw),
		}
		return domain.Review{
			ID:            fmt.Sprintf("%s-%d-approval-%s", repoSlug, pullRequestID, author.ID),
			PullRequestID: pullRequestID,
			RepoSlug:      repoSlug,
			ReviewerID:    author.ID,
			Action:        "approved",
			CreatedAt:     createdAt,
		}, author, true
	case raw.Comment != nil:
		createdAt, err := time.Parse(time.RFC3339, raw.Comment.CreatedOn)
		if err != nil {
			return domain.Review{}, domain.Author{}, false
		}
		author := domain.Author{
			ID:          raw.Comment.User.User.AccountID,
			DisplayName: raw.Comment.User.User.DisplayName,
			Email:       extractEmail(raw.Comment.User.Raw),
		}
		return domain.Review{
			ID:            fmt.Sprintf("%s-%d-comment-%s-%s", repoSlug, pullRequestID, author.ID, createdAt.Format(time.RFC3339)),
			PullRequestID: pullRequestID,
			RepoSlug:      repoSlug,
			ReviewerID:    author.ID,
			Action:        "commented",
			CreatedAt:     createdAt,
		}, author, true
	default:
		return domain.Review{}, domain.Author{}, false
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/normalize/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/normalize/normalize.go internal/normalize/normalize_test.go
git commit -m "feat: add normalize layer for pull requests and reviews"
```
### Task 12: Ingest orchestration with watermarks

**Files:**
- Create: `internal/ingest/ingest.go`
- Test: `internal/ingest/ingest_test.go`

**Interfaces:**
- Consumes: `bitbucket.Client` (Tasks 8-9), `normalize.Commit/FileChanges/PullRequest/Review/IsAllowlisted` (Tasks 10-11), `storage.Store` (Tasks 4-7), `domain.*` (Task 2).
- Produces: `ingest.Syncer{Client *bitbucket.Client; Store *storage.Store; Workspace string; Authors []string}`, `(*Syncer) SyncRepo(ctx context.Context, repoSlug string) error`, `(*Syncer) SyncAll(ctx context.Context, repoSlugs []string) []error` (one error per failed repo; never aborts on first failure). Consumed by `scheduler` (Task 14) and `cmd/server/main.go` (Task 18).

- [ ] **Step 1: Write the failing tests**

```go
// internal/ingest/ingest_test.go
package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/rdwrcloud/repo-one/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"hash":"abc","message":"fix bug","date":"2026-01-05T10:00:00Z","author":{"raw":"Alice <alice@example.com>","user":{"account_id":"acct-1","display_name":"Alice"}}}],"next":""}`))
	})
	mux.HandleFunc("/repositories/rdwrcloud/repo-one/diffstat/abc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"status":"modified","lines_added":3,"lines_removed":1,"new":{"path":"main.go"}}],"next":""}`))
	})
	mux.HandleFunc("/repositories/rdwrcloud/repo-one/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"Add feature","state":"MERGED","created_on":"2026-01-01T09:00:00Z","updated_on":"2026-01-03T09:00:00Z","author":{"raw":"Alice <alice@example.com>","user":{"account_id":"acct-1","display_name":"Alice"}}}],"next":""}`))
	})
	mux.HandleFunc("/repositories/rdwrcloud/repo-one/pullrequests/1/activity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"approval":{"date":"2026-01-02T09:00:00Z","user":{"raw":"Bob <bob@example.com>","user":{"account_id":"acct-2","display_name":"Bob"}}}}],"next":""}`))
	})
	return httptest.NewServer(mux)
}

func TestSyncRepo_PopulatesStoreAndAdvancesWatermark(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := bitbucket.NewClient("svc", "secret")
	client.BaseURL = server.URL
	store := openTestStore(t)

	syncer := &Syncer{Client: client, Store: store, Workspace: "rdwrcloud", Authors: []string{"alice@example.com"}}

	if err := syncer.SyncRepo(context.Background(), "repo-one"); err != nil {
		t.Fatalf("SyncRepo failed: %v", err)
	}

	commits, err := store.ListCommits(storage.Filter{RepoSlug: "repo-one"})
	if err != nil || len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d (err=%v)", len(commits), err)
	}

	changes, err := store.ListFileChanges(storage.Filter{RepoSlug: "repo-one"})
	if err != nil || len(changes) != 1 {
		t.Fatalf("expected 1 file change, got %d (err=%v)", len(changes), err)
	}

	prs, err := store.ListPullRequests(storage.Filter{RepoSlug: "repo-one"})
	if err != nil || len(prs) != 1 {
		t.Fatalf("expected 1 pull request, got %d (err=%v)", len(prs), err)
	}

	reviews, err := store.ListReviews("repo-one", 1)
	if err != nil || len(reviews) != 1 {
		t.Fatalf("expected 1 review, got %d (err=%v)", len(reviews), err)
	}

	authors, err := store.ListAuthors()
	if err != nil {
		t.Fatalf("ListAuthors failed: %v", err)
	}
	for _, a := range authors {
		if a.ID == "acct-1" && !a.Allowlisted {
			t.Error("expected alice (acct-1) to be marked allowlisted")
		}
		if a.ID == "acct-2" && a.Allowlisted {
			t.Error("expected bob (acct-2) to NOT be marked allowlisted")
		}
	}

	syncedAt, err := store.GetRepositorySyncedAt("repo-one")
	if err != nil || syncedAt.IsZero() {
		t.Fatalf("expected watermark to advance, got %v (err=%v)", syncedAt, err)
	}
}

func TestSyncRepo_FailureLeavesWatermarkUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/rdwrcloud/broken-repo/commits", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := bitbucket.NewClient("svc", "secret")
	client.BaseURL = server.URL
	store := openTestStore(t)

	syncer := &Syncer{Client: client, Store: store, Workspace: "rdwrcloud", Authors: []string{"alice@example.com"}}

	if err := syncer.SyncRepo(context.Background(), "broken-repo"); err == nil {
		t.Fatal("expected SyncRepo to return an error on API failure")
	}

	syncedAt, err := store.GetRepositorySyncedAt("broken-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !syncedAt.IsZero() {
		t.Errorf("expected watermark to remain unset after failed sync, got %v", syncedAt)
	}
}

func TestSyncAll_OneRepoFailureDoesNotBlockOthers(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := bitbucket.NewClient("svc", "secret")
	client.BaseURL = server.URL
	store := openTestStore(t)

	syncer := &Syncer{Client: client, Store: store, Workspace: "rdwrcloud", Authors: []string{"alice@example.com"}}

	errs := syncer.SyncAll(context.Background(), []string{"repo-one", "missing-repo"})
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (for missing-repo), got %d: %v", len(errs), errs)
	}

	commits, err := store.ListCommits(storage.Filter{RepoSlug: "repo-one"})
	if err != nil || len(commits) != 1 {
		t.Fatalf("expected repo-one to still sync successfully despite missing-repo failing: %d commits, err=%v", len(commits), err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ingest/...`
Expected: FAIL — package `ingest` / `Syncer` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ingest/ingest.go
package ingest

import (
	"context"
	"fmt"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/domain"
	"git-statistics/internal/normalize"
	"git-statistics/internal/storage"
)

type Syncer struct {
	Client    *bitbucket.Client
	Store     *storage.Store
	Workspace string
	Authors   []string
}

func (s *Syncer) SyncRepo(ctx context.Context, repoSlug string) error {
	since, err := s.Store.GetRepositorySyncedAt(repoSlug)
	if err != nil {
		return fmt.Errorf("reading watermark for %s: %w", repoSlug, err)
	}

	rawCommits, err := s.Client.ListCommits(ctx, s.Workspace, repoSlug, since)
	if err != nil {
		return fmt.Errorf("syncing %s: %w", repoSlug, err)
	}

	latest := since
	for _, raw := range rawCommits {
		commit, author, err := normalize.Commit(repoSlug, raw)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		if !since.IsZero() && !commit.AuthoredAt.After(since) {
			continue
		}

		author.Allowlisted = normalize.IsAllowlisted(author, s.Authors)
		if err := s.Store.UpsertAuthor(author); err != nil {
			return fmt.Errorf("syncing %s: storing author: %w", repoSlug, err)
		}
		if err := s.Store.UpsertCommit(commit); err != nil {
			return fmt.Errorf("syncing %s: storing commit: %w", repoSlug, err)
		}

		rawDiffstat, err := s.Client.GetDiffstat(ctx, s.Workspace, repoSlug, commit.Hash)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		for _, fc := range normalize.FileChanges(repoSlug, commit.Hash, rawDiffstat) {
			if err := s.Store.UpsertFileChange(fc); err != nil {
				return fmt.Errorf("syncing %s: storing file change: %w", repoSlug, err)
			}
		}

		if commit.AuthoredAt.After(latest) {
			latest = commit.AuthoredAt
		}
	}

	rawPRs, err := s.Client.ListPullRequests(ctx, s.Workspace, repoSlug)
	if err != nil {
		return fmt.Errorf("syncing %s: %w", repoSlug, err)
	}
	for _, rawPR := range rawPRs {
		pr, author, err := normalize.PullRequest(repoSlug, rawPR)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		author.Allowlisted = normalize.IsAllowlisted(author, s.Authors)
		if err := s.Store.UpsertAuthor(author); err != nil {
			return fmt.Errorf("syncing %s: storing author: %w", repoSlug, err)
		}
		if err := s.Store.UpsertPullRequest(pr); err != nil {
			return fmt.Errorf("syncing %s: storing pull request: %w", repoSlug, err)
		}

		rawActivity, err := s.Client.ListActivity(ctx, s.Workspace, repoSlug, pr.ID)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		for _, ra := range rawActivity {
			review, reviewer, ok := normalize.Review(repoSlug, pr.ID, ra)
			if !ok {
				continue
			}
			reviewer.Allowlisted = normalize.IsAllowlisted(reviewer, s.Authors)
			if err := s.Store.UpsertAuthor(reviewer); err != nil {
				return fmt.Errorf("syncing %s: storing reviewer: %w", repoSlug, err)
			}
			if err := s.Store.UpsertReview(review); err != nil {
				return fmt.Errorf("syncing %s: storing review: %w", repoSlug, err)
			}
		}
	}

	if err := s.Store.UpsertRepository(domain.Repository{Slug: repoSlug, Workspace: s.Workspace, SyncedAt: latest}); err != nil {
		return fmt.Errorf("syncing %s: advancing watermark: %w", repoSlug, err)
	}
	return nil
}

func (s *Syncer) SyncAll(ctx context.Context, repoSlugs []string) []error {
	var errs []error
	for _, slug := range repoSlugs {
		if err := s.SyncRepo(ctx, slug); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
```

Note: `latest` is initialized to `since` and `time.Time{}` (zero) when this is the repo's first sync, so the very first commit fetched always advances the watermark — the `!since.IsZero() && !commit.AuthoredAt.After(since)` guard only skips already-seen commits on a *second* sync.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ingest/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/ingest.go internal/ingest/ingest_test.go
git commit -m "feat: add ingest orchestration with per-repo watermarks"
```
### Task 13: Metrics - delivery flow, churn, and activity aggregates

**Files:**
- Create: `internal/metrics/metrics.go`
- Test: `internal/metrics/metrics_test.go`

**Interfaces:**
- Consumes: `storage.Store`, `storage.Filter` (Tasks 4-7), `domain.*` (Task 2).
- Produces:
  - `metrics.DeliveryFlow(store *storage.Store, f storage.Filter) ([]metrics.PullRequestFlow, error)` where `PullRequestFlow{PullRequestID int; Title string; CycleTime, TimeToFirstReview, TimeInReview time.Duration}` (only includes merged PRs; `TimeToFirstReview`/`TimeInReview` are zero if there were no reviews).
  - `metrics.ChurnHotspots(store *storage.Store, f storage.Filter) ([]metrics.FileChurn, error)` where `FileChurn{Path string; CommitCount int; LinesChanged int}`, sorted by `LinesChanged` descending.
  - `metrics.CommitsPerAuthor(store *storage.Store, f storage.Filter) ([]metrics.AuthorActivity, error)` where `AuthorActivity{AuthorID, DisplayName string; CommitCount int}`, restricted to allowlisted authors only, sorted by `CommitCount` descending.
- Consumed by `web` (Tasks 15-17).

- [ ] **Step 1: Write the failing tests**

```go
// internal/metrics/metrics_test.go
package metrics

import (
	"path/filepath"
	"testing"
	"time"

	"git-statistics/internal/domain"
	"git-statistics/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeliveryFlow_ComputesCycleTimeAndReviewLag(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	firstReview := created.Add(4 * time.Hour)
	merged := created.Add(48 * time.Hour)

	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertReview(domain.Review{
		ID: "r1", PullRequestID: 1, RepoSlug: "repo-one", ReviewerID: "acct-2",
		Action: "approved", CreatedAt: firstReview,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-one", Title: "WIP", AuthorID: "acct-1",
		State: "OPEN", CreatedAt: created, UpdatedAt: created,
	}))

	flows, err := DeliveryFlow(store, storage.Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("DeliveryFlow failed: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 merged PR in results, got %d", len(flows))
	}
	if flows[0].CycleTime != 48*time.Hour {
		t.Errorf("expected cycle time 48h, got %v", flows[0].CycleTime)
	}
	if flows[0].TimeToFirstReview != 4*time.Hour {
		t.Errorf("expected time to first review 4h, got %v", flows[0].TimeToFirstReview)
	}
	if flows[0].TimeInReview != 44*time.Hour {
		t.Errorf("expected time in review 44h, got %v", flows[0].TimeInReview)
	}
}

func TestChurnHotspots_AggregatesByPathSortedDescending(t *testing.T) {
	store := openTestStore(t)

	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c2", RepoSlug: "repo-one", AuthoredAt: time.Now()}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c1", RepoSlug: "repo-one", Path: "hot.go", LinesAdded: 10, LinesRemoved: 5}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c2", RepoSlug: "repo-one", Path: "hot.go", LinesAdded: 20, LinesRemoved: 0}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c2", RepoSlug: "repo-one", Path: "cold.go", LinesAdded: 1, LinesRemoved: 1}))

	churn, err := ChurnHotspots(store, storage.Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("ChurnHotspots failed: %v", err)
	}
	if len(churn) != 2 {
		t.Fatalf("expected 2 distinct files, got %d", len(churn))
	}
	if churn[0].Path != "hot.go" || churn[0].LinesChanged != 35 || churn[0].CommitCount != 2 {
		t.Errorf("expected hot.go first with 35 lines changed across 2 commits, got %+v", churn[0])
	}
	if churn[1].Path != "cold.go" {
		t.Errorf("expected cold.go second, got %+v", churn[1])
	}
}

func TestCommitsPerAuthor_OnlyAllowlistedSortedDescending(t *testing.T) {
	store := openTestStore(t)

	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice", Allowlisted: true}))
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-2", DisplayName: "Carol", Allowlisted: false}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c2", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c3", RepoSlug: "repo-one", AuthorID: "acct-2", AuthoredAt: time.Now()}))

	activity, err := CommitsPerAuthor(store, storage.Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("CommitsPerAuthor failed: %v", err)
	}
	if len(activity) != 1 {
		t.Fatalf("expected only the allowlisted author (acct-1), got %d entries: %+v", len(activity), activity)
	}
	if activity[0].AuthorID != "acct-1" || activity[0].CommitCount != 2 {
		t.Errorf("unexpected activity: %+v", activity[0])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/metrics/...`
Expected: FAIL — package `metrics` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/metrics/metrics.go
package metrics

import (
	"sort"
	"time"

	"git-statistics/internal/storage"
)

type PullRequestFlow struct {
	PullRequestID     int
	Title             string
	CycleTime         time.Duration
	TimeToFirstReview time.Duration
	TimeInReview      time.Duration
}

type FileChurn struct {
	Path         string
	CommitCount  int
	LinesChanged int
}

type AuthorActivity struct {
	AuthorID    string
	DisplayName string
	CommitCount int
}

func DeliveryFlow(store *storage.Store, f storage.Filter) ([]PullRequestFlow, error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, err
	}

	var flows []PullRequestFlow
	for _, pr := range prs {
		if pr.MergedAt == nil {
			continue
		}
		flow := PullRequestFlow{
			PullRequestID: pr.ID,
			Title:         pr.Title,
			CycleTime:     pr.MergedAt.Sub(pr.CreatedAt),
		}

		reviews, err := store.ListReviews(pr.RepoSlug, pr.ID)
		if err != nil {
			return nil, err
		}
		if len(reviews) > 0 {
			firstReview := reviews[0].CreatedAt
			flow.TimeToFirstReview = firstReview.Sub(pr.CreatedAt)
			flow.TimeInReview = pr.MergedAt.Sub(firstReview)
		}
		flows = append(flows, flow)
	}
	return flows, nil
}

func ChurnHotspots(store *storage.Store, f storage.Filter) ([]FileChurn, error) {
	changes, err := store.ListFileChanges(f)
	if err != nil {
		return nil, err
	}

	type agg struct {
		commits map[string]bool
		lines   int
	}
	byPath := map[string]*agg{}
	for _, c := range changes {
		a, ok := byPath[c.Path]
		if !ok {
			a = &agg{commits: map[string]bool{}}
			byPath[c.Path] = a
		}
		a.commits[c.CommitHash] = true
		a.lines += c.LinesAdded + c.LinesRemoved
	}

	churn := make([]FileChurn, 0, len(byPath))
	for path, a := range byPath {
		churn = append(churn, FileChurn{Path: path, CommitCount: len(a.commits), LinesChanged: a.lines})
	}
	sort.Slice(churn, func(i, j int) bool { return churn[i].LinesChanged > churn[j].LinesChanged })
	return churn, nil
}

func CommitsPerAuthor(store *storage.Store, f storage.Filter) ([]AuthorActivity, error) {
	commits, err := store.ListCommits(f)
	if err != nil {
		return nil, err
	}
	authors, err := store.ListAuthors()
	if err != nil {
		return nil, err
	}

	allowlisted := map[string]string{}
	for _, a := range authors {
		if a.Allowlisted {
			allowlisted[a.ID] = a.DisplayName
		}
	}

	counts := map[string]int{}
	for _, c := range commits {
		if _, ok := allowlisted[c.AuthorID]; ok {
			counts[c.AuthorID]++
		}
	}

	activity := make([]AuthorActivity, 0, len(counts))
	for authorID, count := range counts {
		activity = append(activity, AuthorActivity{AuthorID: authorID, DisplayName: allowlisted[authorID], CommitCount: count})
	}
	sort.Slice(activity, func(i, j int) bool { return activity[i].CommitCount > activity[j].CommitCount })
	return activity, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/metrics/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metrics/metrics.go internal/metrics/metrics_test.go
git commit -m "feat: add metrics engine for delivery flow, churn, and activity"
```
### Task 14: Scheduler with manual trigger

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Test: `internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: nothing concrete - takes a `SyncFunc func(ctx context.Context)` callback so it doesn't need to import `ingest` directly (keeps the dependency direction one-way: `main` wires `ingest.Syncer.SyncAll` into this).
- Produces: `scheduler.New(interval time.Duration, sync SyncFunc) *Scheduler`, `(*Scheduler) Start(ctx context.Context)` (runs in the caller's goroutine - caller should call `go scheduler.Start(ctx)`), `(*Scheduler) TriggerNow()` (non-blocking; queues an immediate sync if one isn't already pending). Consumed by `cmd/server/main.go` (Task 18) and the "Sync now" HTTP handler (Task 17).

- [ ] **Step 1: Write the failing tests**

```go
// internal/scheduler/scheduler_test.go
package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_TicksOnInterval(t *testing.T) {
	var count int32
	s := New(20*time.Millisecond, func(ctx context.Context) {
		atomic.AddInt32(&count, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	go s.Start(ctx)
	defer cancel()

	time.Sleep(70 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	if got := atomic.LoadInt32(&count); got < 2 {
		t.Errorf("expected at least 2 ticks in 70ms at 20ms interval, got %d", got)
	}
}

func TestScheduler_TriggerNowRunsImmediately(t *testing.T) {
	done := make(chan struct{}, 1)
	s := New(time.Hour, func(ctx context.Context) {
		done <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)

	s.TriggerNow()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected TriggerNow to cause an immediate sync run")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scheduler/...`
Expected: FAIL — package `scheduler` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scheduler/scheduler.go
package scheduler

import (
	"context"
	"time"
)

type SyncFunc func(ctx context.Context)

type Scheduler struct {
	interval time.Duration
	sync     SyncFunc
	trigger  chan struct{}
}

func New(interval time.Duration, sync SyncFunc) *Scheduler {
	return &Scheduler{
		interval: interval,
		sync:     sync,
		trigger:  make(chan struct{}, 1),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync(ctx)
		case <-s.trigger:
			s.sync(ctx)
		}
	}
}

func (s *Scheduler) TriggerNow() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scheduler/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scheduler/scheduler.go internal/scheduler/scheduler_test.go
git commit -m "feat: add periodic scheduler with manual trigger"
```
### Task 15: Web - filter parsing, layout, and Activity dashboard

**Files:**
- Create: `internal/web/web.go`
- Create: `internal/web/handlers.go`
- Create: `internal/web/templates/layout.html`
- Create: `internal/web/templates/activity.html`
- Test: `internal/web/web_test.go`

**Interfaces:**
- Consumes: `storage.Store`, `storage.Filter` (Task 4); `metrics.CommitsPerAuthor` (Task 13); `scheduler.Scheduler` (Task 14, referenced here only to define the shared `Handler` struct field; its `TriggerNow` is wired up in Task 17).
- Produces: `web.Handler{Store *storage.Store; Scheduler *scheduler.Scheduler; Repos []string}`, `web.NewHandler(store *storage.Store, sched *scheduler.Scheduler, repos []string) *Handler`, `(*Handler) Routes() http.Handler`, `parseFilter(r *http.Request) (storage.Filter, error)` (parses `repo`, `author`, `from`, `to` query params; `from`/`to` are `YYYY-MM-DD`). Route `/activity` is rendered in this task; `/`, `/delivery-flow`, `/churn`, `/sync` are added in Tasks 16-17.

- [ ] **Step 1: Write the failing test**

```go
// internal/web/web_test.go
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git-statistics/internal/domain"
	"git-statistics/internal/scheduler"
	"git-statistics/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestHandler(t *testing.T, store *storage.Store) http.Handler {
	t.Helper()
	sched := scheduler.New(time.Hour, func(ctx context.Context) {})
	h := NewHandler(store, sched, []string{"repo-one"})
	return h.Routes()
}

func TestActivityDashboard_ListsAllowlistedAuthorCommitCounts(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice", Allowlisted: true}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Errorf("expected response to mention Alice, got: %s", body)
	}
	if !strings.Contains(body, "Activity") {
		t.Errorf("expected response to mention the Activity dashboard title, got: %s", body)
	}
}

func TestActivityDashboard_FiltersByRepo(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice", Allowlisted: true}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c2", RepoSlug: "repo-two", AuthorID: "acct-1", AuthoredAt: time.Now()}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/activity?repo=repo-one", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), ">1<") {
		t.Errorf("expected commit count of 1 after repo filter, got: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/...`
Expected: FAIL — package `web` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/web/web.go
package web

import (
	"embed"
	"html/template"
	"net/http"

	"git-statistics/internal/scheduler"
	"git-statistics/internal/storage"
)

//go:embed templates/*.html
var templateFS embed.FS

type Handler struct {
	Store     *storage.Store
	Scheduler *scheduler.Scheduler
	Repos     []string
	templates *template.Template
}

func NewHandler(store *storage.Store, sched *scheduler.Scheduler, repos []string) *Handler {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	return &Handler{Store: store, Scheduler: sched, Repos: repos, templates: tmpl}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/activity", http.StatusFound)
	})
	mux.HandleFunc("/activity", h.handleActivity)
	return mux
}
```

```go
// internal/web/handlers.go
package web

import (
	"net/http"
	"time"

	"git-statistics/internal/metrics"
	"git-statistics/internal/storage"
)

func parseFilter(r *http.Request) (storage.Filter, error) {
	f := storage.Filter{
		RepoSlug: r.URL.Query().Get("repo"),
		AuthorID: r.URL.Query().Get("author"),
	}
	if from := r.URL.Query().Get("from"); from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err != nil {
			return storage.Filter{}, err
		}
		f.From = t
	}
	if to := r.URL.Query().Get("to"); to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err != nil {
			return storage.Filter{}, err
		}
		f.To = t
	}
	return f, nil
}

type filterFormData struct {
	Repos          []string
	SelectedRepo   string
	SelectedAuthor string
	From           string
	To             string
}

func (h *Handler) filterForm(r *http.Request) filterFormData {
	q := r.URL.Query()
	return filterFormData{
		Repos:          h.Repos,
		SelectedRepo:   q.Get("repo"),
		SelectedAuthor: q.Get("author"),
		From:           q.Get("from"),
		To:             q.Get("to"),
	}
}

type activityPageData struct {
	Filter filterFormData
	Rows   []metrics.AuthorActivity
}

func (h *Handler) handleActivity(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := metrics.CommitsPerAuthor(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := activityPageData{Filter: h.filterForm(r), Rows: rows}
	if err := h.templates.ExecuteTemplate(w, "activity.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

```html
<!-- internal/web/templates/layout.html -->
{{define "nav"}}
<nav>
  <a href="/activity">Activity</a> |
  <a href="/delivery-flow">Delivery Flow</a> |
  <a href="/churn">Code Churn</a>
  <form method="POST" action="/sync" style="display:inline">
    <button type="submit">Sync now</button>
  </form>
</nav>
{{end}}

{{define "filters"}}
<form method="GET">
  <label>Repo:
    <select name="repo">
      <option value="">All</option>
      {{range .Repos}}<option value="{{.}}" {{if eq . $.SelectedRepo}}selected{{end}}>{{.}}</option>{{end}}
    </select>
  </label>
  <label>Author: <input type="text" name="author" value="{{.SelectedAuthor}}"></label>
  <label>From: <input type="date" name="from" value="{{.From}}"></label>
  <label>To: <input type="date" name="to" value="{{.To}}"></label>
  <button type="submit">Apply filters</button>
</form>
{{end}}
```

```html
<!-- internal/web/templates/activity.html -->
{{template "nav" .}}
<h1>Activity</h1>
{{template "filters" .Filter}}
<table>
  <thead><tr><th>Author</th><th>Commits</th></tr></thead>
  <tbody>
    {{range .Rows}}
    <tr><td>{{.DisplayName}}</td><td>{{.CommitCount}}</td></tr>
    {{end}}
  </tbody>
</table>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/web/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/web.go internal/web/handlers.go internal/web/templates/layout.html internal/web/templates/activity.html internal/web/web_test.go
git commit -m "feat: add web layer with filters and activity dashboard"
```
### Task 16: Web - Delivery Flow dashboard

**Files:**
- Modify: `internal/web/web.go`
- Modify: `internal/web/handlers.go`
- Create: `internal/web/templates/delivery_flow.html`
- Modify: `internal/web/web_test.go`

**Interfaces:**
- Consumes: `metrics.DeliveryFlow` (Task 13); `filterFormData`, `parseFilter`, `(*Handler).filterForm` (Task 15).
- Produces: `/delivery-flow` route rendering `metrics.PullRequestFlow` rows with durations formatted as hours.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/web/web_test.go

func TestDeliveryFlowDashboard_ShowsMergedPRsOnly(t *testing.T) {
	store := openTestStore(t)
	created := time.Now().Add(-72 * time.Hour)
	merged := created.Add(48 * time.Hour)
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-one", Title: "WIP", AuthorID: "acct-1",
		State: "OPEN", CreatedAt: created, UpdatedAt: created,
	}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/delivery-flow", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Add feature") {
		t.Errorf("expected merged PR title in response, got: %s", body)
	}
	if strings.Contains(body, "WIP") {
		t.Errorf("expected open PR to be excluded from delivery flow, got: %s", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/...`
Expected: FAIL — 404 on `/delivery-flow` (route not registered).

- [ ] **Step 3: Write minimal implementation**

```go
// in internal/web/web.go, add inside Routes():
	mux.HandleFunc("/delivery-flow", h.handleDeliveryFlow)
```

```go
// append to internal/web/handlers.go

type deliveryFlowRow struct {
	Title            string
	CycleTimeHours   float64
	FirstReviewHours float64
	InReviewHours    float64
}

type deliveryFlowPageData struct {
	Filter filterFormData
	Rows   []deliveryFlowRow
}

func (h *Handler) handleDeliveryFlow(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flows, err := metrics.DeliveryFlow(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]deliveryFlowRow, 0, len(flows))
	for _, f := range flows {
		rows = append(rows, deliveryFlowRow{
			Title:            f.Title,
			CycleTimeHours:   f.CycleTime.Hours(),
			FirstReviewHours: f.TimeToFirstReview.Hours(),
			InReviewHours:    f.TimeInReview.Hours(),
		})
	}

	data := deliveryFlowPageData{Filter: h.filterForm(r), Rows: rows}
	if err := h.templates.ExecuteTemplate(w, "delivery_flow.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

```html
<!-- internal/web/templates/delivery_flow.html -->
{{template "nav" .}}
<h1>Delivery Flow</h1>
{{template "filters" .Filter}}
<table>
  <thead><tr><th>Pull Request</th><th>Cycle Time (h)</th><th>Time to First Review (h)</th><th>Time in Review (h)</th></tr></thead>
  <tbody>
    {{range .Rows}}
    <tr><td>{{.Title}}</td><td>{{printf "%.1f" .CycleTimeHours}}</td><td>{{printf "%.1f" .FirstReviewHours}}</td><td>{{printf "%.1f" .InReviewHours}}</td></tr>
    {{end}}
  </tbody>
</table>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/web/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/web.go internal/web/handlers.go internal/web/templates/delivery_flow.html internal/web/web_test.go
git commit -m "feat: add delivery flow dashboard"
```

### Task 17: Web - Code Churn dashboard and Sync now endpoint

**Files:**
- Modify: `internal/web/web.go`
- Modify: `internal/web/handlers.go`
- Create: `internal/web/templates/churn.html`
- Modify: `internal/web/web_test.go`

**Interfaces:**
- Consumes: `metrics.ChurnHotspots` (Task 13); `Handler.Scheduler.TriggerNow` (Task 14).
- Produces: `/churn` route; `/sync` route (`POST`, calls `Scheduler.TriggerNow()` and redirects back to the referring page).

- [ ] **Step 1: Write the failing tests**

```go
// append to internal/web/web_test.go

func TestChurnDashboard_ShowsTopFiles(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthoredAt: time.Now()}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c1", RepoSlug: "repo-one", Path: "hot.go", LinesAdded: 50, LinesRemoved: 10}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/churn", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hot.go") {
		t.Errorf("expected hot.go in churn dashboard, got: %s", rec.Body.String())
	}
}

func TestSyncEndpoint_TriggersSchedulerAndRedirects(t *testing.T) {
	store := openTestStore(t)
	triggered := make(chan struct{}, 1)
	sched := scheduler.New(time.Hour, func(ctx context.Context) { triggered <- struct{}{} })
	h := NewHandler(store, sched, []string{"repo-one"})
	handler := h.Routes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Start(ctx)

	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	req.Header.Set("Referer", "/activity")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/activity" {
		t.Errorf("expected redirect to /activity, got %q", rec.Header().Get("Location"))
	}

	select {
	case <-triggered:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected sync to be triggered")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web/...`
Expected: FAIL — 404 on `/churn` and `/sync`.

- [ ] **Step 3: Write minimal implementation**

```go
// in internal/web/web.go, add inside Routes():
	mux.HandleFunc("/churn", h.handleChurn)
	mux.HandleFunc("/sync", h.handleSync)
```

```go
// append to internal/web/handlers.go

type churnPageData struct {
	Filter filterFormData
	Rows   []metrics.FileChurn
}

func (h *Handler) handleChurn(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := metrics.ChurnHotspots(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := churnPageData{Filter: h.filterForm(r), Rows: rows}
	if err := h.templates.ExecuteTemplate(w, "churn.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	h.Scheduler.TriggerNow()
	redirectTo := r.Header.Get("Referer")
	if redirectTo == "" {
		redirectTo = "/activity"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}
```

```html
<!-- internal/web/templates/churn.html -->
{{template "nav" .}}
<h1>Code Churn</h1>
{{template "filters" .Filter}}
<table>
  <thead><tr><th>File</th><th>Commits</th><th>Lines Changed</th></tr></thead>
  <tbody>
    {{range .Rows}}
    <tr><td>{{.Path}}</td><td>{{.CommitCount}}</td><td>{{.LinesChanged}}</td></tr>
    {{end}}
  </tbody>
</table>
```

Add `"git-statistics/internal/scheduler"` and `"context"` to `internal/web/web_test.go`'s imports if not already present from Task 15 (Task 15's test file already imports both).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/web.go internal/web/handlers.go internal/web/templates/churn.html internal/web/web_test.go
git commit -m "feat: add code churn dashboard and sync now endpoint"
```
### Task 18: Wire main.go end-to-end

**Files:**
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `config.Load` (Task 3), `storage.Open` (Task 4), `bitbucket.NewClient` (Task 8), `ingest.Syncer` (Task 12), `scheduler.New` (Task 14), `web.NewHandler` (Task 15).
- Produces: a fully wired `main()` that starts the scheduler in a goroutine and serves the web handler on `:8080`. No new exported interface - this is the composition root.

- [ ] **Step 1: Replace main.go with the full wiring**

```go
// cmd/server/main.go
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/config"
	"git-statistics/internal/ingest"
	"git-statistics/internal/scheduler"
	"git-statistics/internal/storage"
	"git-statistics/internal/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config file")
	dbPath := flag.String("db", "git-statistics.db", "path to SQLite database file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	store, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer store.Close()

	client := bitbucket.NewClient(cfg.BitbucketUsername, cfg.BitbucketAppPassword)
	syncer := &ingest.Syncer{
		Client:    client,
		Store:     store,
		Workspace: cfg.Bitbucket.Workspace,
		Authors:   cfg.Authors,
	}

	interval := time.Duration(cfg.SyncIntervalMinutes) * time.Minute
	sched := scheduler.New(interval, func(ctx context.Context) {
		for _, err := range syncer.SyncAll(ctx, cfg.Bitbucket.Repos) {
			log.Printf("sync error: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Start(ctx)

	handler := web.NewHandler(store, sched, cfg.Bitbucket.Repos)

	mux := newMux()
	mux.Handle("/", handler.Routes())

	log.Printf("git-statistics server listening on :8080 (config=%s, db=%s)", *configPath, *dbPath)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
```

Note: `mux.Handle("/", handler.Routes())` registers `web`'s router as the catch-all on the outer mux. Go's `http.ServeMux` pattern matching means `/healthz` registered directly on the outer mux still wins for that exact path because it is a more specific pattern than `/`.

- [ ] **Step 2: Verify the existing health endpoint test still passes**

Run: `go test ./cmd/server/...`
Expected: PASS (the `TestHealthEndpoint` test from Task 1 still calls `newMux()` directly, which is unchanged in behavior).

- [ ] **Step 3: Verify the whole project builds**

Run: `go build ./...`
Expected: succeeds with no errors.

- [ ] **Step 4: Manually verify the server starts locally**

Run (PowerShell):
```
$env:BITBUCKET_USERNAME = "dummy"
$env:BITBUCKET_APP_PASSWORD = "dummy"
go run ./cmd/server --config testdata/config.yaml --db (Join-Path $env:TEMP "git-statistics-manual-test.db")
```
Expected: log line `git-statistics server listening on :8080 ...` and `http://localhost:8080/` redirects to `/activity`, rendering an empty Activity table (no sync has run against a real Bitbucket account yet - that's expected; Task 19 covers verifying the sync path against a mocked server). Stop the server with Ctrl+C.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire config, storage, ingest, scheduler, and web into main"
```

### Task 19: End-to-end integration test

**Files:**
- Create: `internal/integration/integration_test.go`

**Interfaces:**
- Consumes: every package from Tasks 4-17 (`bitbucket`, `ingest`, `storage`, `metrics`, `web`, `scheduler`).
- Produces: nothing new - this is a test-only package proving the full path (fixture Bitbucket payloads -> sync -> normalize -> metrics -> SQLite -> HTTP dashboard response) works together, satisfying the design spec's "one integration test exercising the full path" testing requirement.

- [ ] **Step 1: Write the integration test**

```go
// internal/integration/integration_test.go
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/ingest"
	"git-statistics/internal/scheduler"
	"git-statistics/internal/storage"
	"git-statistics/internal/web"
)

func TestFullPath_SyncThenDashboardsReflectData(t *testing.T) {
	bbMux := http.NewServeMux()
	bbMux.HandleFunc("/repositories/rdwrcloud/sample-repo/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"hash":"abc","message":"fix bug","date":"2026-01-05T10:00:00Z","author":{"raw":"Alice <alice@example.com>","user":{"account_id":"acct-1","display_name":"Alice"}}}],"next":""}`))
	})
	bbMux.HandleFunc("/repositories/rdwrcloud/sample-repo/diffstat/abc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"status":"modified","lines_added":12,"lines_removed":3,"new":{"path":"main.go"}}],"next":""}`))
	})
	bbMux.HandleFunc("/repositories/rdwrcloud/sample-repo/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"Add login","state":"MERGED","created_on":"2026-01-01T09:00:00Z","updated_on":"2026-01-02T09:00:00Z","author":{"raw":"Alice <alice@example.com>","user":{"account_id":"acct-1","display_name":"Alice"}}}],"next":""}`))
	})
	bbMux.HandleFunc("/repositories/rdwrcloud/sample-repo/pullrequests/1/activity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"values":[{"approval":{"date":"2026-01-01T15:00:00Z","user":{"raw":"Bob <bob@example.com>","user":{"account_id":"acct-2","display_name":"Bob"}}}}],"next":""}`))
	})
	bitbucketServer := httptest.NewServer(bbMux)
	defer bitbucketServer.Close()

	store, err := storage.Open(filepath.Join(t.TempDir(), "integration.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	client := bitbucket.NewClient("svc", "secret")
	client.BaseURL = bitbucketServer.URL

	syncer := &ingest.Syncer{
		Client:    client,
		Store:     store,
		Workspace: "rdwrcloud",
		Authors:   []string{"alice@example.com"},
	}

	if err := syncer.SyncRepo(context.Background(), "sample-repo"); err != nil {
		t.Fatalf("SyncRepo failed: %v", err)
	}

	sched := scheduler.New(time.Hour, func(ctx context.Context) {})
	handler := web.NewHandler(store, sched, []string{"sample-repo"})
	server := handler.Routes()

	t.Run("activity dashboard shows synced author commit count", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/activity", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Alice") {
			t.Errorf("expected Alice in activity dashboard: %s", rec.Body.String())
		}
	})

	t.Run("delivery flow dashboard shows merged PR cycle time", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/delivery-flow", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Add login") {
			t.Errorf("expected merged PR title in delivery flow dashboard: %s", rec.Body.String())
		}
	})

	t.Run("churn dashboard shows changed file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/churn", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "main.go") {
			t.Errorf("expected main.go in churn dashboard: %s", rec.Body.String())
		}
	})

	t.Run("re-running sync does not duplicate data", func(t *testing.T) {
		if err := syncer.SyncRepo(context.Background(), "sample-repo"); err != nil {
			t.Fatalf("second SyncRepo failed: %v", err)
		}
		commits, err := store.ListCommits(storage.Filter{RepoSlug: "sample-repo"})
		if err != nil {
			t.Fatalf("ListCommits failed: %v", err)
		}
		if len(commits) != 1 {
			t.Fatalf("expected exactly 1 commit after re-sync, got %d", len(commits))
		}
	})
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./internal/integration/...`
Expected: PASS

If it fails, the failure points to a wiring mismatch between two packages built in earlier tasks (e.g. a route name or a field name that drifted) - fix the mismatch in the package that's wrong relative to this plan's Interfaces sections, not in the test.

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: PASS across every package.

- [ ] **Step 4: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add end-to-end integration test covering sync through dashboards"
```