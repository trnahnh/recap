# Roadmap — Recap

## Phase 0 — Manual prototype
- Create a few records manually
- Connect two AI coding tools
- Confirm one tool can use decisions created by another

## Phase 1a — PostgreSQL infrastructure
- Docker Compose setup (`postgres:16`), daemon auto-manages container
  lifecycle (see ARCHITECTURE_DECISIONS.md ADR-002)
- Migrations tooling: `golang-migrate` (Go — see ADR-007)
- Daemon with connection pool (fixed default 10, overridable)
- `recap export` / `import` (`pg_dump`/`pg_restore`)

## Phase 1b — Core CRUD
- Project and record models, with foreign keys and cascade rules
- Create, read, update, delete operations
- Basic CLI commands

## Phase 2 — Tool connection
- MCP server; connect Claude Code and Codex CLI
  - **Blocker:** no MCP tool schema is defined yet — nothing to build
    against until this is written.
  - **Blocker:** non-MCP hook contract (for tools without MCP support)
    is unspecified — trigger conditions, call signature, failure
    behavior all TBD.
- Shared record format across tools

## Phase 3 — Decision capture
- Save command, structured draft generation, developer approval
- Capture Git branch, commit, changed files (Git-detected, not
  self-reported by the AI tool)
- Session end is explicit-only: triggered solely by `recap save`, no idle
  timeout or process-exit heuristic (resolved, see SYSTEM_DESIGN.md
  "Session boundary")

## Phase 4 — Context search
- `tsvector` full-text search, GIN index, single scored SQL query merging
  `ts_rank` with boosts (see SYSTEM_DESIGN.md "Search")
- Project/branch/file filters, relevance + recency ranking
- Context size limit: default 5 records **and** ~2000-token ceiling,
  whichever hits first, both configurable (resolved, see SYSTEM_DESIGN.md
  "Search")

## Phase 5 — Safety and quality
- Secret filtering (regex-based first pass — known limitation, see
  ARCHITECTURE_DECISIONS.md ADR-006)
- Replaced-decision handling, conflicting-record warnings
- Prompt injection testing, incorrect-summary testing
- Small fixed eval set for retrieval-quality regression checks (see
  METRICS.md)

## Phase 6 — Packaging
- Simple installation command, setup documentation, example projects
- Publish as open source

## Later phases (explicitly out of v1 scope)
- Local web dashboard
- Team sharing, user authentication
- PostgreSQL → managed/remote deployment option
- Embedding-based search
- Additional AI coding tools
