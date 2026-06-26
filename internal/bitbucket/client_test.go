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
