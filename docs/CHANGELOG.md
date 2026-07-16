# Changelog — Recap

All notable changes to this project are recorded here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Changed
- **Storage backend switched from SQLite to PostgreSQL.** Original PRD
  draft specified SQLite as the default local store; revised after
  technical review to PostgreSQL for proper concurrent-write handling
  (row-level locking vs. SQLite's file-level locking). See
  `ARCHITECTURE_DECISIONS.md` ADR-001.
- Full-text search mechanism updated from SQLite FTS5 to PostgreSQL
  `tsvector`/`tsquery` with GIN index accordingly (ADR-004).

### Added
- Persistent daemon + connection pool architecture (replaces the
  assumption of direct per-process SQLite file access). See ADR-003.
- `recap export` / `import` promoted from "later" to a Phase 1a
  requirement, since local-only storage without a backup path was
  flagged as a real portability risk.

### Open (not yet decided — tracked in ROADMAP.md)
- Docker-based vs. embedded PostgreSQL install path (ADR-002)
- Language choice for daemon/CLI (Python / TypeScript / Go / Rust)
- Session-end trigger definition
- MCP tool schema (see API_REFERENCE.md)

---

## [0.0-draft] — Initial PRD

- First product summary, problem statement, and MVP scope drafted.
- SQLite proposed as default storage (later revised — see Unreleased).
- Initial data model (projects, sessions, records, alternatives,
  record_files, record_relationships) drafted.
