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

## ADR-002: Docker vs. embedded Postgres (OPEN — Blocker)

**Status:** Not yet decided. Must be resolved before schema/migration
work starts.

**Option A — Dockerized Postgres:** simplest to build and maintain, but
the "install in under 5 minutes" claim becomes conditional on Docker
already being installed.

**Option B — Embedded Postgres:** no external dependency to ask of the
user, but adds real packaging complexity per OS/architecture and is more
work to get right.

**Impact if left undecided:** blocks Phase 1a entirely — the daemon can't
be built without knowing which Postgres it's talking to.

## ADR-003: Persistent daemon with connection pool

**Decision:** All reads/writes go through a long-running local daemon
holding a small (5–10) connection pool. No CLI command or MCP call opens
its own DB connection.

**Why:** Short-lived CLI processes opening a fresh Postgres connection
each time adds real latency and risks connection exhaustion under
concurrent tool use — unlike SQLite, where a direct file open was cheap.

**Open:** pool size is a guess, not based on measured concurrent usage.

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

## Unresolved / not yet an ADR

- Language choice for the daemon/CLI (Python, TypeScript, Go, Rust) — see
  ROADMAP.md open questions.
- MCP tool schema — nothing formal defined yet; blocks API_REFERENCE.md
  from being filled in.
- Non-MCP hook contract for tools like Codex CLI — completely
  unspecified (what triggers it, what it calls, silent-failure behavior).
