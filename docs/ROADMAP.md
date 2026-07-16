# Roadmap — Recap

## Phase 0 — Manual prototype
- Create a few records manually
- Connect two AI coding tools
- Confirm one tool can use decisions created by another

## Phase 1a — PostgreSQL infrastructure
- Docker Compose setup **or** embedded Postgres — decision required first
  (see ARCHITECTURE_DECISIONS.md ADR-002, currently a Blocker)
- Migrations tooling in place (e.g. `sqlx migrate`, `golang-migrate`,
  `alembic`, depending on language choice — see Open Questions)
- Daemon with connection pool
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
- Capture Git branch, commit, changed files
- **Blocker:** "session end" trigger is undefined (explicit command? idle
  timeout? process exit?) — this phase can't be built until it's decided.

## Phase 4 — Context search
- `tsvector` full-text search, GIN index
- Project/branch/file filters, relevance + recency ranking
- Context size limits — needs a concrete N/token budget, not just "small"

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

## Open questions

- **Docker vs. embedded Postgres** — Blocker, resolve before Phase 1a.
- Should the daemon auto-manage the Postgres container lifecycle, or does
  the developer own that separately?
- Should saving always require an explicit command, or can Recap
  auto-draft when a session ends (depends on session-end definition
  above)?
- How much context should be loaded at session start — concrete limit
  not yet set.
- Should file changes be detected via Git or reported by the AI tool
  itself?
- How should two contradicting decisions be surfaced/resolved?
- Language choice: Python, TypeScript, Go, or Rust — affects migrations
  tooling choice in Phase 1a.
