# Architecture Decisions — Recap

## ADR-001: PostgreSQL over SQLite

**Decision:** Use PostgreSQL as the only storage backend, not SQLite.

**Why:** SQLite's file-level locking is a poor fit for two AI tools
(e.g. Claude Code + Codex CLI) writing to the same project concurrently.
Postgres gives proper row-level locking, real transactions, and headroom
for scale that SQLite doesn't.

**Cost:** SQLite required zero infrastructure — a file. Postgres requires
a running service, which changes the install story materially (see
ADR-002).

## ADR-002: Dockerized Postgres (`postgres:16` via Docker Compose)

**Decision:** Use Dockerized Postgres for v1, not embedded Postgres.
The daemon auto-manages the container lifecycle (`recap init`/`start`
shells out to `docker compose up`, stop tears it down) rather than
leaving that to the developer.

**Why:** Simplest to build and maintain. Manual container ownership would
reintroduce the friction Docker was meant to remove, undercutting the
"under 5 minutes" install goal. The "under 5 minutes" claim is now
explicitly conditional on Docker already being installed — document this
plainly rather than implying a zero-dependency install.

**Rejected:** Embedded Postgres — real per-OS/architecture packaging
complexity with no validated demand for a zero-dependency install over a
one-time Docker prerequisite.

**Follow-up:** daemon needs Docker socket/CLI access to manage the
container — document this as a runtime dependency, not just an install-
time one.

## ADR-003: Persistent daemon with connection pool

**Decision:** All reads/writes go through a long-running local daemon
holding a small (5–10) connection pool. No CLI command or MCP call opens
its own DB connection.

**Why:** Short-lived CLI processes opening a fresh Postgres connection
each time adds real latency and risks connection exhaustion under
concurrent tool use — unlike SQLite, where a direct file open was cheap.

**Pool size:** fixed default of 10, overridable via config. Not load-tested
— deliberately: this is a local, single-developer tool where realistic
concurrent load is 2–4 tool processes, not a server workload. Revisit with
real usage data (via `recap status`/logs) rather than building load-test
infrastructure for a scale this tool doesn't operate at.

## ADR-004: tsvector/GIN full-text search, no embeddings in v1

**Decision:** Use PostgreSQL's built-in `tsvector`/`tsquery` with a GIN
index for v1 search. No embedding model, local or remote.

**Why:** Keeps v1 dependency-free (no embedding API key, no local model
runtime) while still giving real keyword search with ranking.

**Future-proofing:** schema should not block adding a `pgvector` column
later if tsvector search proves insufficient.

## ADR-005: Records are drafts until approved

**Decision:** AI-generated decision summaries are never written as
trusted memory directly — they're stored as `draft` status and require
explicit developer approval before being retrievable as context.

**Why:** Directly mitigates the "incorrect records" and "trust across
tools" risks — an AI-generated summary shouldn't silently become fact for
the next tool to consume.

**Known gap:** approval happens once, at write time. There's no mechanism
for a second tool to later flag that an approved record no longer matches
reality (see RISKS in SYSTEM_DESIGN.md).

**Related decision — conflicting records:** when retrieval finds two
active records that contradict each other, surface both to the developer/
AI tool explicitly rather than auto-resolving (e.g. auto-superseding the
older one). Auto-resolution risks silently hiding a contradiction from the
next tool — exactly the failure mode this ADR exists to prevent.

## ADR-006: Secret filtering is regex-based (known limitation)

**Decision:** Strip likely secrets (API keys, tokens, passwords) via
regex pattern matching before storage, for known key formats.

**Why chosen anyway:** zero dependency, fast, catches the common cases
(AWS keys, common token prefixes, etc.)

**Explicitly known limitation:** this will miss non-standard secret
formats and should never be presented to users as a guarantee. Flagged
as **Fix-before-v1 priority** in the technical review — this is the
highest-consequence gap in the whole system given it touches real
project code.

**v1 mitigation (two layers):**
1. Content regex matching (as above) for known key formats.
2. Filename/path denylist — refuse to ingest content sourced from
   `.env`, `*.pem`, `credentials.json`, and similar credential-shaped
   paths, regardless of content match.

The draft-approval gate (ADR-005) remains the real backstop — regex and
the denylist are first-pass filters, not a substitute for human review
before a record becomes retrievable. Document this combination as a
known, partial mitigation in user-facing docs, never as a guarantee.

## ADR-007: Go as the implementation language

**Decision:** Daemon, CLI, and MCP server are implemented in Go.

**Why:** Compiles to a single static binary per OS, which directly serves
the Phase 6 "simple install command" goal — no bundled runtime needed
(unlike a TypeScript/Node or PyInstaller-bundled Python option). Solid
Postgres driver (`pgx`), mature migrations tooling (`golang-migrate`),
and goroutines fit a connection-pooled daemon naturally.

**Rejected:**
- **TypeScript** — best alignment with the MCP reference-server ecosystem,
  but weaker single-binary packaging story (needs a bundled Node runtime).
- **Rust** — same single-binary packaging win as Go, but a steeper dev
  velocity cost not justified by any known performance constraint.
- **Python** — best DX for Postgres/migrations, but weakest fit for
  "ship one binary" — needs PyInstaller-style bundling, adds runtime
  fragility to the install story.

**Impact:** migrations tooling is `golang-migrate` (Phase 1a); MCP SDK
choice should target Go's MCP SDK.

## ADR-008: Concurrent supersede resolution

**Decision:** When superseding a record, acquire the row lock with
`SELECT ... FOR UPDATE` inside a transaction, then re-check
`records.status` before writing:

- If still `active`, proceed and supersede normally.
- If already `superseded` (a concurrent writer won the race), fail clean
  with a specific error — e.g. `record 42 was already superseded by
  record 47 — rerun against the current record` — rather than silently
  overwriting.

**Why:** Reuses `status` as the version signal, so no new version column
or schema change is needed — same lock ADR-003's pool already supports,
plus one status check and one error message. Two tools operating
concurrently on the same project is the core use case this product is
built for, not an edge case to defer past v1.

## Unresolved / not yet an ADR

- MCP tool schema — nothing formal defined yet; blocks API_REFERENCE.md
  from being filled in.
- Non-MCP hook contract for tools like Codex CLI — completely
  unspecified (what triggers it, what it calls, silent-failure behavior).
