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
- `bitbucket.repos` — repo slugs to sync. Need ≥1.
- `sync_interval_minutes` — sync loop period. Must be > 0.
- `authors` — allowlisted author IDs tracked. Need ≥1.

## Secrets (env vars, NOT in YAML)

```
BITBUCKET_USERNAME=your-username
BITBUCKET_APP_PASSWORD=your-app-password
```

Both required or `config.Load` errors out. Get app password from Bitbucket → Personal settings → App passwords (needs repo read + PR read scopes).

## SQLite DB

No manual create step. `storage.Open(path)` opens (or creates) sqlite file and runs schema (`CREATE TABLE IF NOT EXISTS ...`) automatically — tables: `repositories`, `authors`, `commits`, `file_changes`, `pull_requests`, `reviews`.

Default path `git-statistics.db`, override `-db`.

## Run

```
BITBUCKET_USERNAME=alice BITBUCKET_APP_PASSWORD=xxxx \
  go run ./cmd/server -config config.yaml -db git-statistics.db
```

Windows PowerShell:

```powershell
$env:BITBUCKET_USERNAME = "alice"
$env:BITBUCKET_APP_PASSWORD = "xxxx"
go run ./cmd/server -config config.yaml -db git-statistics.db
```

Server listen `:8080`. Health check `GET /healthz`.

## Flow on start
1. Load YAML config + env secrets, validate.
2. Open/create SQLite DB, run schema.
3. Build Bitbucket client + syncer.
4. Start background scheduler (sync every `sync_interval_minutes`).
5. Serve dashboard routes via `internal/web`.
