# Deploy — Recap

Recap is local-only. "Deploy" here means installing and running it on
a single developer machine — there is no hosted/cloud version in v1.

## Prerequisites

- Git
- Either Docker (if ADR-002 resolves to Dockerized Postgres) **or** no
  extra prerequisite (if embedded Postgres is chosen instead)

> This decision is currently open — see ARCHITECTURE_DECISIONS.md
> ADR-002. Nothing below can be finalized until it's made.

## Install (Docker path, if chosen)

```
recap init
```

This will:
1. Detect the current Git repository.
2. Start a local `postgres:16` container via Docker Compose, bound to
   `127.0.0.1` only.
3. Generate a random per-install database credential, stored at
   `~/.recap/config` with `0600` permissions.
4. Run migrations to create the schema.
5. Start the Recap daemon (persistent, connection-pooled).
6. Offer to configure supported AI tools (Claude Code, Codex CLI).

## Install (embedded path, if chosen instead)

TBD — depends on ADR-002. Would bundle a Postgres binary with the CLI
itself, removing the Docker dependency at the cost of per-OS packaging
work.

## Security defaults on install

- Postgres binds to `127.0.0.1` only — enforced at daemon startup, not
  just documented.
- Password auth required (`trust` auth explicitly disabled).
- No external network calls at runtime; the only network activity is the
  one-time Docker image pull at install (Docker path only).

## Uninstall / data removal

- `recap delete <project>` removes all records for a project.
- Full removal: stop the daemon, remove the Postgres container/data
  volume (Docker path) or the embedded data directory, delete
  `~/.recap/`.

## Backup before uninstall or machine switch

```
recap export
```

Wraps `pg_dump`. Keep the export file somewhere outside `~/.recap/`
if you're about to wipe the machine.
