# Setup & Run

## Build

```
go build ./...
```

## Config file

App want YAML config (default path `config.yaml`, override `-config`).

```yaml
bitbucket:
  workspace: my-workspace
  repos:
    - repo-one
    - repo-two
sync_interval_minutes: 30
authors:
  - alice
  - bob
```

Fields:
- `bitbucket.workspace` — Bitbucket workspace slug. Required.
- `bitbucket.repos` — repo slugs to sync. Need ≥1. Use `"*"` (sole entry) to sync every repo in workspace.
- `sync_interval_minutes` — sync loop period. Must be > 0.
- `authors` — allowlisted author IDs (emails) tracked. Need ≥1. Use `"*"` (sole entry) to allowlist everyone.

```yaml
bitbucket:
  workspace: my-workspace
  repos:
    - "*"
sync_interval_minutes: 30
authors:
  - "*"
```

## Secrets (env vars, NOT in YAML)

```
BITBUCKET_API_TOKEN=your-api-token
```

Required or `config.Load` errors out. Create an API token in Bitbucket → Personal settings → API tokens (needs repo read + PR read scopes). Sent as `Authorization: Bearer <token>` on every request.

## SQLite DB

No manual create step. `storage.Open(path)` opens (or creates) sqlite file and runs schema (`CREATE TABLE IF NOT EXISTS ...`) automatically — tables: `repositories`, `authors`, `commits`, `file_changes`, `pull_requests`, `reviews`.

Default path `git-statistics.db`, override `-db`.

## Run

```
BITBUCKET_API_TOKEN=xxxx \
  go run ./cmd/server -config config.yaml -db git-statistics.db
```

Windows PowerShell:

```powershell
$env:BITBUCKET_API_TOKEN = "xxxx"
go run ./cmd/server -config config.yaml -db git-statistics.db
```

Server listen `:8080`. Health check `GET /healthz`.

## Flow on start
1. Load YAML config + env secrets, validate.
2. Open/create SQLite DB, run schema.
3. Build Bitbucket client (token auth) + syncer.
4. If `repos: ["*"]`, list all repos in the workspace via the Bitbucket API and use that resolved list everywhere below.
5. Start background scheduler (sync every `sync_interval_minutes`).
6. Serve dashboard routes via `internal/web`.

## Run in MCP mode (for agents)

Same `-config`/`-db` flags, plus `-mcp`. Instead of starting the HTTP listener and sync scheduler, this runs an MCP server over stdio (`github.com/modelcontextprotocol/go-sdk`) so an external agent (VS Code Copilot, Claude Code, etc.) can query the same metrics directly.

```
go run ./cmd/server -mcp -config config.yaml -db git-statistics.db
```

There's no `:8080` to hit in this mode — the only interface is stdio, so an MCP client launches it as a subprocess (`command`/`args` pointing at this binary with `-mcp`). It's read-only against whatever the `-db` file already has: this mode never starts the Bitbucket client or syncer, so run the app in normal HTTP mode at least once first to populate the database.

Exposes:
- 7 tools mirroring the dashboards: `delivery_flow`, `summary_stats`, `distributions`, `churn_hotspots`, `commits_per_author`, `breakdown_by_repository`, `breakdown_by_author`
- 2 tools for PR status (not on any dashboard): `pr_status_by_author`, `pr_status_by_repo`
- 3 curated prompts bundling those tools into investigation workflows: `investigate_progress`, `team_activity_summary`, `pr_health_check`

See `docs/solutions/documentation-gaps/running-the-mcp-server.md` for full tool/prompt parameters and an MCP client config example.

$env:BITBUCKET_EMAIL = "anatolyb@radware.com"