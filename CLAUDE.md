# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This repository currently contains only [README.md](README.md), which is a product/feature spec for "Git Analytics App" — no code has been written yet. There are no build, lint, or test commands because no implementation exists. When code is added, update this file with the real commands and architecture.

## What this project is

An internal application that analyzes Git activity (commits, PRs, reviews, merges) across repositories and surfaces engineering flow metrics — code churn, PR cycle time, review lag, merge rate, activity heatmaps — in a team-level dashboard. It is explicitly **not** meant to rank individual developers, replace project management tooling, do deep static code analysis, or enforce process. See [README.md](README.md) for the full spec: core metrics, dashboards (Overview, Activity, Delivery flow, Code churn), filters, data sources (GitHub/GitLab/Bitbucket), data model, and MVP scope.

## Architecture direction

The spec's suggested architecture is: an ingestion service (pulls from Git provider APIs) → a normalization layer (maps provider events to a common schema) → a metrics engine (computes aggregates/derived metrics) → an API layer (serves dashboards/exports) → a web UI (charts and tables).

When implementing this:
- **Backend services** (ingestion, normalization, metrics engine, API layer): prefer **Go**.
- **Agent/automation pieces** (e.g. sync orchestration, anomaly detection, AI-generated summaries): prefer **MCP-based tools** or the **Astra framework** over building bespoke integrations from scratch.

## MCP Configuration

MCP (Model Context Protocol) servers are configured in `.claude/mcp.json` to enable:
- **docs**: Query Go standard library and third-party package documentation
- **serena**: Symbol-aware project navigation and editing (find/rename symbols, references, structured edits) — prefer it over ad-hoc grep/sed for non-trivial refactors
- Shell commands (go, git, npm, make, sqlite3) are pre-approved for faster workflows

See `.claude/settings.json` for project-specific settings and permissions.

## Documented Solutions

`docs/solutions/` — documented solutions to past problems (bugs, best practices, workflow patterns), organized by category with YAML frontmatter (`module`, `tags`, `problem_type`). Relevant when implementing or debugging in documented areas.

`CONCEPTS.md` — shared domain vocabulary (entities, named processes, status concepts) such as Cycle Time, Delivery Flow, and Allowlisted Author. Relevant when orienting to the codebase or discussing domain concepts.
