# Deployment Checklist — Recap (pre-v1 release)

Use this before cutting a v1 release. Items marked **BLOCKER** must be
resolved; items marked **should** are strongly recommended but won't
outright break the product if missed.

## Infrastructure
- [ ] Docker Compose (`postgres:16`), daemon-managed lifecycle —
      decided, ADR-002; still needs to be built
- [ ] Migrations tooling (`golang-migrate`, ADR-007) wired into
      `recap init`
- [ ] Daemon starts, holds connection pool, survives basic kill/restart
- [ ] `recap export` / `import` tested against a real project's data

## Security
- [ ] **BLOCKER** — Postgres bind-to-localhost verified in code (not just
      documented), daemon refuses to start if bound elsewhere
- [ ] Password auth confirmed active, `trust` auth confirmed disabled
- [ ] Per-install credential generation and file permission (`0600`)
      verified
- [ ] **BLOCKER** — confirm no unsanitized user input (branch names, file
      paths, search queries) reaches raw SQL or a shell command
- [ ] Secret-filtering regex set **and** filename/path denylist
      (`.env`, `*.pem`, `credentials.json`, etc. — ADR-006) reviewed and
      documented as a known, partial mitigation (not a guarantee) in
      user-facing docs

## Data model
- [ ] Unique constraint on `projects.project_key` in place (prevents
      silent project merges)
- [ ] Cascade/delete behavior confirmed for every foreign key, including
      `record_relationships`
- [ ] `records.confidence` implemented as enum (`low`/`medium`/`high` —
      decided, see ARCHITECTURE_DECISIONS.md)

## Concurrency
- [ ] Concurrent supersede race — resolution decided (`FOR UPDATE` +
      status recheck, fail clean on conflict — ADR-008); still needs to
      be implemented and tested
- [ ] Two simultaneous tool sessions on the same project tested manually

## Core behavior
- [ ] "Session end" trigger — decided (explicit-only via `recap save`,
      see SYSTEM_DESIGN.md "Session boundary"); still needs to be
      implemented consistently across Claude Code and Codex CLI adapters
- [ ] MCP tool schema written and versioned
- [ ] Non-MCP hook contract written (trigger, call signature, failure
      behavior) for tools without MCP support

## Observability
- [ ] **BLOCKER** — basic logging exists for daemon start/stop, write
      failures, and retrieval errors (see OBSERVABILITY.md)
- [ ] A documented way to inspect/repair a bad DB state beyond
      delete-everything

## Quality
- [ ] Retrieval-quality eval set built and passing (see METRICS.md)
- [ ] Prompt-injection test cases run against stored free-text fields
- [ ] Context size limit has a concrete default value, not just "small"

## Packaging
- [ ] Install completes and is timed for real (confirm the 5-minute
      claim under the chosen install path)
- [ ] README states known limitations plainly: regex-only secret
      detection, single-developer/single-machine scope, no auto
      re-validation of stale records
