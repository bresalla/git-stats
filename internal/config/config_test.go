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
	t.Setenv("BITBUCKET_EMAIL", "svc@example.com")
	t.Setenv("BITBUCKET_API_TOKEN", "secret-token")
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
	if cfg.BitbucketAPIToken != "secret-token" {
		t.Errorf("expected API token from env, got %q", cfg.BitbucketAPIToken)
	}
}

func TestLoad_MissingCredentials(t *testing.T) {
	t.Setenv("BITBUCKET_API_TOKEN", "")
	path := writeTempConfig(t, validYAML)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
}

func TestLoad_NoRepos(t *testing.T) {
	t.Setenv("BITBUCKET_EMAIL", "svc@example.com")
	t.Setenv("BITBUCKET_API_TOKEN", "secret-token")
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
	t.Setenv("BITBUCKET_EMAIL", "svc@example.com")
	t.Setenv("BITBUCKET_API_TOKEN", "secret-token")
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

func TestLoad_WildcardReposAndAuthors(t *testing.T) {
	t.Setenv("BITBUCKET_EMAIL", "svc@example.com")
	t.Setenv("BITBUCKET_API_TOKEN", "secret-token")
	path := writeTempConfig(t, `
bitbucket:
  workspace: rdwrcloud
  repos:
    - "*"
sync_interval_minutes: 30
authors:
  - "*"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Bitbucket.Repos) != 1 || cfg.Bitbucket.Repos[0] != "*" {
		t.Errorf("expected wildcard repo, got %v", cfg.Bitbucket.Repos)
	}
	if len(cfg.Authors) != 1 || cfg.Authors[0] != "*" {
		t.Errorf("expected wildcard author, got %v", cfg.Authors)
	}
}
