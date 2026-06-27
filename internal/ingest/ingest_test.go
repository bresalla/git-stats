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
		_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"Add feature","state":"MERGED","created_on":"2026-01-01T09:00:00Z","updated_on":"2026-01-03T09:00:00Z","author":{"account_id":"acct-1","display_name":"Alice"}}],"next":""}`))
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

	client := bitbucket.NewClient("test@example.com", "test-token")
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
	if prs[0].AuthorID != "acct-1" {
		t.Errorf("expected pull request AuthorID to be acct-1, got %q", prs[0].AuthorID)
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

	client := bitbucket.NewClient("test@example.com", "test-token")
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

	client := bitbucket.NewClient("test@example.com", "test-token")
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
