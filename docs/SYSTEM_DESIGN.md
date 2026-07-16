# System Design — Recap

## Components

```
Claude Code ──┐
              │
Codex CLI ────┼── MCP, commands, or hooks
              │
              ▼
       Recap daemon (persistent, connection-pooled)
       ├── Tool integration layer
       ├── Record validation
       ├── Context search (tsvector)
       ├── Secret filtering
       ├── CLI
       └── PostgreSQL (local)
```

PostgreSQL requires a persistent daemon holding a small connection pool
(5–10 connections). CLI commands and MCP calls go through the daemon —
nothing opens its own database connection directly.

> **Open (Blocker):** pool size is currently a guess, not tied to any
> measured concurrent-tool-count assumption. Needs load testing before v1.

## Recap service

Handles: creating projects, saving/validating/searching records, formatting
context, updating/deleting records, communicating with supported AI tools.
Exposes MCP tools, an optional local REST API, and CLI commands.

## Storage — PostgreSQL

Default and only database (no SQLite fallback). Two install paths, **one
must be chosen before schema work starts**:

- **Dockerized Postgres** (`postgres:16` via Docker Compose) — simplest to
  maintain, requires Docker present.
- **Embedded Postgres** — no external dependency, more packaging work per
  OS/architecture.

Requirements regardless of path:
- Bind to `127.0.0.1` only, never `0.0.0.0` — enforced at daemon startup,
  not just documented as a rule.
- Password auth (not `trust`) in `pg_hba.conf`.
- Random per-install credential generated at `recap init`, stored in a
  local config file with `0600` permissions.

## Search

No embeddings in v1. Full-text search uses `tsvector`/`tsquery` with a GIN
index on `title`, `summary`, `rationale`. Ranking combines `ts_rank` with
boosts for: current project, files touched, task keyword match, active
status, recency.

> **Open (Fix-before-v1):** the exact merge of `ts_rank` with the
> non-text boosts (single scored SQL query vs. application-side merge) is
> not yet decided. The wrong choice won't scale past a few thousand
> records.

Schema should leave room for a `pgvector` column later without a painful
migration, even though embeddings are out of scope for v1.

## Request flows

**Saving a record**
Developer finishes task → AI tool creates structured draft → Recap
validates → secret filtering → developer approves/edits → stored in
PostgreSQL.

**Loading context**
Developer starts new task → AI tool sends task + project info → Recap
searches (tsvector + filters) → excludes replaced/invalid records → selects
a small set → developer reviews → context handed to AI tool, explicitly
framed as reference data, not instructions.

## Known concurrency risk

Two tools open on the same project simultaneously both write through the
daemon. Postgres handles this far better than SQLite would, but the actual
transaction/locking strategy for "mark record superseded" must be
deliberate:

- Wrap check-then-write sequences in a single transaction.
- Use `SELECT ... FOR UPDATE` when updating a record under concurrent
  access.

> **Blocker, unresolved:** two sessions finishing at the same time and
> both trying to supersede the same prior record has no described
> resolution yet.

## Session boundary (unresolved — blocks Phase 3/4)

What constitutes "session end" (explicit command / idle timeout / tool
process exit) is not decided. Multiple phases assume it's already solved.
Must be resolved before implementation starts on decision capture.
