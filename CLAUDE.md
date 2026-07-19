# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project status — read this first

Recap is **pre-implementation**. What exists today is the PRD and the `docs/`
set; no schema has been created yet. Do not assume a stack, scaffold a repo
shape, or pick a database driver on your own — check
`docs/ARCHITECTURE_DECISIONS.md` and `docs/ROADMAP.md` first, and if a
decision marked **OPEN** there hasn't been resolved, ask rather than guessing.

The two decisions previously called out here as explicitly open are now
resolved (see ARCHITECTURE_DECISIONS.md ADR-002, ADR-007):

- **Language:** Go — single-binary packaging, `pgx`/`golang-migrate` for
  Postgres/migrations.
- **Postgres install:** Dockerized (`postgres:16` via Docker Compose),
  daemon auto-manages the container lifecycle.

Remaining genuinely open items: the MCP tool schema and the non-MCP hook
contract — see `docs/ARCHITECTURE_DECISIONS.md` ("Unresolved / not yet an
ADR") and `docs/API_REFERENCE.md`.

## What Recap is

A local, PostgreSQL-backed memory system for AI coding tools. It saves
structured decision records (task, chosen approach, rejected approaches +
reasons, files touched) from one AI tool and serves them to another tool on
the same project, so a developer switching between Claude Code and Codex CLI
doesn't re-explain context that's already been settled.

Three logical components, per `docs/SYSTEM_DESIGN.md`:

1. **Daemon** — persistent process, holds the PostgreSQL connection pool.
   Nothing else opens its own DB connection.
2. **MCP server** — exposes `save_record` / `get_context` (schemas in
   `docs/API_REFERENCE.md`) to MCP-capable tools.
3. **Hook adapter** — a JSON-over-stdin/stdout CLI contract for tools without
   MCP support (see `docs/API_REFERENCE.md`, `Non-MCP Hook Contract`).

## Non-negotiable invariants

These come directly from the PRD (`docs/ARCHITECTURE_DECISIONS.md`,
`docs/SYSTEM_DESIGN.md` §12) and hold regardless of which language is chosen:

- **Drafts require approval.** An AI-generated decision summary is never
  written as trusted, retrievable memory directly — it's `draft` status
  until a human approves it. Do not add a code path that skips this.
- **Local-only, no silent network calls.** No data leaves the machine by
  default. If you add any outbound call (e.g. an optional embedding API),
  it must be opt-in and documented, never a default.
- **Localhost-only binding, enforced in code.** The daemon must check its
  own PostgreSQL bind address at startup and refuse to start if it isn't
  `127.0.0.1` — this is a startup assertion, not just a config comment.
- **Parameterized queries only.** No user-controlled input (branch names,
  file paths, search queries) is ever interpolated into a SQL string or a
  shell command. The hook contract passes structured JSON for this reason —
  don't reintroduce shell-string interpolation to "simplify" an adapter.
- **Secret filtering is regex-based and explicitly partial.** Don't present
  it in code comments, docs, or user-facing text as a guarantee — it's a
  first-pass mitigation, documented as such in `docs/ARCHITECTURE_DECISIONS.md`
  ADR-006.
- **Migrations are append-only** once the schema exists — never edit a
  shipped migration, always generate a new one.
- **Session-end trigger is explicit only, for v1.** Decision capture is
  triggered by an explicit `recap save` (or tool-side equivalent), not an
  idle timeout or process-exit heuristic. Don't add auto-detection without
  flagging it as a scope change against §7.1 of the PRD.

## Commands

Go module `github.com/trnahnh/recap`. Requires Go 1.25+ and Docker (with the
`docker compose` plugin) running — the daemon shells out to Compose to manage
the `postgres:16` container. Run all commands from the repo root.

```
go mod tidy            # sync dependencies
go build ./...         # build every package
go vet ./...           # static checks (no separate linter wired yet)
go test ./...          # run tests (suite not written yet)
go run ./cmd/recap ... # run the CLI without installing
go build -o bin/recap ./cmd/recap   # produce the single binary
```

CLI lifecycle (Phase 1a; later phases add save/list/search/etc.):

```
recap init     # generate config (0600, random credential), start Postgres, migrate
recap start    # start Postgres for an already-initialized install
recap stop     # stop the container (named volume recap_pgdata persists data)
recap status   # report daemon/DB health
```

All commands accept `--config <path>` to override the default config location
(`os.UserConfigDir()/recap/config.json`). Migrations are embedded in the binary
(`migrations/*.sql` via `//go:embed`) and applied by the daemon on start — no
separate `migrate` CLI needed.

Layout: `cmd/recap` (CLI entry), `internal/config` (config + credential),
`internal/db` (pool, loopback assertion, migration runner), `internal/daemon`
(container lifecycle + orchestration), `migrations` (SQL + embed).

## Reference docs

Read the relevant one before implementing in that area — these are the
source of truth, not this file:

- `docs/README.md` — product story, install, usage, current status.
- `docs/SYSTEM_DESIGN.md` — components, storage, search/ranking, request
  flows, concurrency handling.
- `docs/ARCHITECTURE_DECISIONS.md` — ADRs: Postgres over SQLite, Docker vs.
  embedded (open), persistent daemon + pool, tsvector search, drafts-require-
  approval, regex-based secret filtering (known limitation).
- `docs/API_REFERENCE.md` — MCP tool schemas (`save_record`, `get_context`),
  non-MCP hook contract, CLI command list, shared record data format.
- `docs/OBSERVABILITY.md` — logging plan, `recap status`, repair path for a
  bad DB state.
- `docs/ONBOARDING.md` — setup flow, daily usage, exporting/moving machines.
- `docs/ROADMAP.md` — phased plan (Phase 0 manual prototype through Phase 6
  packaging) plus what's resolved vs. still open.
- `docs/METRICS.md` — success metrics, the retrieval-quality eval set
  (not yet built — build it before Phase 4 is considered done).
- `docs/DEPLOY.md` / `docs/DEPLOYMENT_CHECKLIST.md` — install path, security
  defaults on install, the pre-release checklist (Blockers must be resolved
  before cutting v1).
- `docs/CHANGELOG.md` — what changed between PRD revisions and why (e.g. the
  SQLite → PostgreSQL switch).

## Code style

Keep the codebase comment-free. Do not add `//`, `#`, or SQL `--` comments to
explain what code does — write self-documenting code (clear names, small
functions) instead. The only comments allowed are ones the toolchain requires,
e.g. `//go:embed` directives and build tags. The "why" behind a non-obvious
decision belongs in `docs/` (an ADR or SYSTEM_DESIGN.md), not inline. This
applies to all source and config files (Go, SQL migrations, YAML, Dockerfiles).

## Git

- Do not add AI attribution to commits or PRs — no `Co-Authored-By: Claude`
  trailer, no `Claude-Session` trailer, no "Generated with Claude" line. Commits
  are authored solely by the human git user.
- Split work into atomic commits: one logical change per commit, ordered so
  dependencies land before the code that uses them.
- Only commit or push when explicitly asked.

## Markdown & docs

Keep `docs/*.md` consistent with the existing set: one H1 per file, sections
mirroring the pattern already established (resolved items called out plainly,
open items marked as needing confirmation rather than silently decided).

## Session log

`SESSION.md` (gitignored) records the latest, most recent finished state
locally — a compact snapshot of where the code is right now, not a running
history. It is distinct from `docs/ROADMAP.md`, which is the forward plan (to
be broken down into dividable tasks). Overwrite it to reflect the current
finished state after each implementation task, before reporting the work done —
this is what lets work survive a context-window compaction. Keep it short; do
not accumulate stale entries.
