# Observability — Recap

> **Status: gap.** The PRD does not currently specify any logging or
> monitoring plan. This directly undermines the PRD's own "silent
> degradation" risk mitigation (§14 of the PRD) — if the daemon or
> Postgres isn't running, or a write silently fails, nothing here yet
> tells the developer that happened. Flagged as a **Blocker** in the
> technical review. This file is a starting proposal, not settled design.

## Why this matters specifically for Recap

Recap runs as a background daemon the developer doesn't actively
watch. If it silently fails to record a decision, or silently returns
stale/incorrect context, the entire value proposition (avoiding repeated
context) breaks without anyone noticing — arguably worse than the tool
not existing, since it creates false confidence.

## Minimum proposed logging (to be built, not yet implemented)

- Daemon lifecycle: start, stop, crash/restart, Postgres connection
  established/lost.
- Every write attempt: success/failure, with failure reason (validation
  error, secret detected and stripped, DB error).
- Every retrieval: query terms, number of records returned, latency.
- Daemon health check: developer-facing `recap status` command
  reporting whether the daemon is up and Postgres is reachable — this
  should exist before v1 ships, not be a "later" item, since it's the
  direct fix for silent degradation.

## Minimum proposed error surface

- Failures should be visible to the CLI command that triggered them, not
  swallowed. A write that fails should tell the developer it failed, not
  proceed as if it succeeded.
- MCP calls that fail should return a clear error to the calling tool,
  not an empty/ambiguous result that looks like "no relevant records
  found."

## Repair path (currently undefined)

There's no described way to inspect or repair a corrupted/inconsistent
DB state beyond deleting all data. At minimum, before v1:
- A `recap doctor` or similar command to check schema integrity,
  orphaned rows, and connection health.
- Clear documentation on what "delete everything and re-init" actually
  loses (i.e., point to `recap export` as the way to avoid needing
  this at all).

## Explicitly out of scope for v1

- Centralized/remote log aggregation — this is a local single-developer
  tool, logs stay local (a local file under `~/.recap/logs/` is
  sufficient).
- Metrics dashboards — informal `recap status` output is enough for
  v1; see METRICS.md for the retrieval-quality eval, which is separate
  from operational logging.
