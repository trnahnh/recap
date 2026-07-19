# API Reference — Recap

> **Status: incomplete.** The PRD references an MCP interface and CLI
> commands but does not define a formal MCP tool schema or REST API
> contract. This blocks Phase 2 (Tool connection) from being built as-is
> — flagged as a **Blocker** in the technical review. Sections below mark
> what's confirmed vs. what needs to be written before implementation.

## CLI commands (confirmed in PRD)

```
recap init                    # register project, start daemon
recap status                  # daemon/DB health (proposed, see OBSERVABILITY.md)
recap save                    # create a draft record from current session
recap list                    # list records
recap search "<query>"        # keyword search
recap show <id>               # view a record
recap edit <id>                # edit a record
recap delete <id>              # delete a record
recap archive <id>             # archive a record
recap export                  # pg_dump wrapper
recap import                  # pg_restore wrapper
```

## MCP interface — NOT YET DEFINED

The PRD states Recap should "provide an MCP interface where
supported" but no tool names, input schemas, or output formats are
specified anywhere. This needs to be written before Claude Code / Claude
Desktop integration can start. At minimum, needs:

- A tool for **writing** a decision record (draft creation)
- A tool for **querying** context given a task description + project
- Confirmed input/output JSON schemas for both
- Versioning approach for when the schema changes

## Non-MCP hook contract — NOT YET DEFINED

For tools without MCP support (e.g. Codex CLI today), the PRD mentions
"small hooks or scripts" with no further detail. Needs, before Phase 2:

- What triggers the hook (file watch? explicit CLI call from within the
  tool? shell wrapper around the tool's invocation?)
- What the hook actually calls (local REST endpoint? direct CLI
  subprocess call?)
- Defined behavior on failure — silent skip is explicitly called out as
  unacceptable given the "silent degradation" risk in the PRD.

## Local REST API — optional, unspecified

The PRD mentions "a small local REST API if needed" as an option but
does not commit to endpoints. If built, should mirror the CLI commands
above (project registration, record CRUD, search) rather than
introducing a separate surface to maintain.

## Data format for records (from PRD §10 — this part IS specified)

Implemented in `internal/model` (issue #1). The structs map
`migrations/000001_init.up.sql` one-for-one; the shape below is the shared
representation both MCP tools and hook adapters read/write. A `?` marks a
nullable field, omitted from JSON when absent.

```
record {
  id                 # uuid, DB-assigned
  project_id         # uuid
  session_id?        # uuid, nullable — no session model in v1
  record_type: decision | failed_attempt | constraint | discovery | open_question
  title
  task
  summary
  chosen_approach?
  rationale?
  status: draft | active | superseded | archived | invalid
  confidence?        # float in [0,1], nullable
  created_by
  created_at
  updated_at
  alternatives:  [{ id, record_id, approach, result?, reason?, position }]
  files:         [{ id, record_id, file_path, commit_hash? }]
  relationships: [{ id, record_id, target_record_id, relationship_type, created_at }]
}
```

Enum values equal the Postgres enum labels byte-for-byte. `confidence` is a
nullable float constrained to 0..1 (finalized in issue #1). Child collections
carry their own `id` and owning `record_id` so a single alternative, file, or
relationship can be addressed for edit/delete, and serialize as `[]` when
empty, never `null` — a superset of the original PRD sketch, which omitted
child ids and `alternatives.position`.
