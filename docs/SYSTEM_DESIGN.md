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
(fixed default of 10, overridable via config). CLI commands and MCP calls
go through the daemon — nothing opens its own database connection
directly.

Pool size is not load-tested — deliberately: this is a local, single-
developer tool where realistic concurrent load is 2–4 tool processes, not
a server workload. Revisit with real usage data rather than building
load-test infrastructure ahead of need (see ADR-003).

## Recap service

Handles: creating projects, saving/validating/searching records, formatting
context, updating/deleting records, communicating with supported AI tools.
Exposes MCP tools, an optional local REST API, and CLI commands.

## Storage — PostgreSQL

Default and only database (no SQLite fallback). Dockerized Postgres
(`postgres:16` via Docker Compose), per ADR-002. The daemon auto-manages
the container lifecycle — `recap init`/`start` shells out to
`docker compose up`, stop tears it down — so the developer never
interacts with Docker directly.

Requirements:
- Bind to `127.0.0.1` only, never `0.0.0.0` — enforced at daemon startup,
  not just documented as a rule.
- Password auth (not `trust`) in `pg_hba.conf`.
- Random per-install credential generated at `recap init`, stored in a
  local config file with `0600` permissions.

## Search

No embeddings in v1. Full-text search uses `tsvector`/`tsquery` with a GIN
index on `title`, `summary`, `rationale`. Ranking combines `ts_rank` with
boosts for: current project, files touched, task keyword match, active
status, recency — computed as a single scored SQL query (combined score
in SQL, `ORDER BY` it, `LIMIT N`), not merged in application code. All
boost inputs are already columns on the record, so there's no reason to
pull full result sets into app memory to resort; this also avoids a
second place ranking logic can drift out of sync with the schema.

Context returned to the AI tool is capped by a configurable record count
(default 5) **and** a fixed token ceiling (~2000 tokens) — whichever
limit is hit first truncates the result. Both knobs are adjustable via
`recap config` (`context.max_records`, `context.max_tokens`). When the
token ceiling truncates before `max_records` is reached, this is surfaced
explicitly (e.g. "only 3 of your configured 8 fit within the token
ceiling") rather than silently returning fewer records than expected.

Schema should leave room for a `pgvector` column later without a painful
migration, even though embeddings are out of scope for v1.

## Request flows

**Saving a record**
Developer finishes task → AI tool creates structured draft → Recap
validates → secret filtering (regex + filename/path denylist) → files
touched are Git-detected (`git diff`/`status` against the session-start
commit, not self-reported by the AI tool) → developer approves/edits →
stored in PostgreSQL.

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

**Resolved (ADR-008):** acquire the row lock with `SELECT ... FOR UPDATE`,
re-check `records.status` before writing. If still `active`, proceed. If
already `superseded` by a concurrent writer, fail clean with a specific
error rather than silently overwriting. Reuses `status` as the version
signal — no new schema needed.

## Session boundary

**Resolved:** session end is explicit-only for v1 — triggered solely by
`recap save` (or a tool-side equivalent command), never an idle timeout
or process-exit heuristic. This removes the need for a "session boundary"
detection mechanism entirely: there is nothing to detect, only a command
to call. Auto-detection is out of scope for v1 and would be a scope
change requiring explicit sign-off (see CLAUDE.md).
