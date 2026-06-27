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
		_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"Add login","state":"MERGED","created_on":"2026-01-01T09:00:00Z","updated_on":"2026-01-02T09:00:00Z","author":{"account_id":"acct-1","display_name":"Alice"}}],"next":""}`))
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

	client := bitbucket.NewClient("test@example.com", "test-token")
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
