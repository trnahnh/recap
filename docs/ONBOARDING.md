# Onboarding — Recap

## Who this is for

A developer who:
- Works on one or more software projects
- Uses at least two AI coding tools (e.g. Claude Code + Codex CLI)
- Frequently switches between them
- Wants to avoid repeating project context every time
- Does not want project information sent to another company

v1 targets **one developer, one computer**.

## Setup

1. Open a Git project.
2. Run:
   ```
   recap init
   ```
3. Recap registers the project and connects to a local Dockerized
   PostgreSQL instance (see ARCHITECTURE_DECISIONS.md ADR-002).

> Install target is under 5 minutes — conditional on Docker already being
> installed.

## Daily workflow

1. Work with Claude Code as normal.
2. At an important checkpoint, Claude creates a decision draft:
   task, chosen approach + reason, rejected approaches + reasons, files
   touched.
3. You approve, edit, or reject the draft. Only approved records become
   trusted memory.
4. Switch to Codex CLI in the same repo.
5. Codex asks Recap for relevant decisions; Recap returns the
   context saved from Claude.
6. Review the returned context before it's used — it's shown to you, not
   silently injected.

**Not supported in v1:** website-based LLM chats (chatgpt.com, claude.ai,
gemini.google.com).

## Reviewing your project history

```
recap list
recap search "notification retry"
recap show 15
recap edit 15
recap delete 15
recap archive 15
```

## Exporting / moving machines

```
recap export [--out <file>]
recap import <file>
```

`export` writes `./recap-export-<timestamp>.dump` by default; pass `--out`
to choose the path. `import` takes the dump file as its argument.

Since storage is local-only, this is the only way to carry your decision
history to another machine. Do this before wiping or switching computers.

## What Recap won't do

- Won't save your general preferences or writing style — project
  decisions only.
- Won't read your ChatGPT/Claude/Gemini web chat history.
- Won't send anything off your machine by default.
- Won't trust an AI-generated summary until you approve it.
