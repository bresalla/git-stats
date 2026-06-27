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
	mcpMode := flag.Bool("mcp", false, "run in MCP server mode over stdio")
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

	if *mcpMode {
		if err := runMCPServer(store); err != nil {
			log.Fatalf("running MCP server: %v", err)
		}
		return
	}

	client := bitbucket.NewClient(cfg.BitbucketEmail, cfg.BitbucketAPIToken)
	syncer := &ingest.Syncer{
		Client:    client,
		Store:     store,
		Workspace: cfg.Bitbucket.Workspace,
		Authors:   cfg.Authors,
	}

	repoSlugs := cfg.Bitbucket.Repos
	if isWildcard(repoSlugs) {
		resolved, err := client.ListRepositories(context.Background(), cfg.Bitbucket.Workspace)
		if err != nil {
			log.Fatalf("listing repositories: %v", err)
		}
		repoSlugs = resolved
	}

	interval := time.Duration(cfg.SyncIntervalMinutes) * time.Minute
	sched := scheduler.New(interval, func(ctx context.Context) {
		for _, err := range syncer.SyncAll(ctx, repoSlugs) {
			log.Printf("sync error: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Start(ctx)

	handler := web.NewHandler(store, sched, repoSlugs)

	mux := newMux()
	mux.Handle("/", handler.Routes())

	log.Printf("git-statistics server listening on :8080 (config=%s, db=%s)", *configPath, *dbPath)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func isWildcard(slugs []string) bool {
	for _, slug := range slugs {
		if slug == "*" {
			return true
		}
	}
	return false
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
