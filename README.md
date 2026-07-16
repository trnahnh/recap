# Recap

Local memory for AI coding tools. Save a decision in Claude Code, pick it up in Codex CLI — without re-explaining yourself.

### Why

We kept re-explaining the same rejected approach to whatever AI tool we opened that day. Recap saves the decision, not the whole conversation: what you tried, what you rejected and why, what you picked, which files it touched. Everything stays local. Nothing gets trusted until you approve it.

### Install

```
recap init
```

Detects your Git repo, starts a local PostgreSQL instance, sets up the daemon. Under 5 minutes if Docker's already installed.

### Usage

```bash
recap save                    # save a decision from the current session
recap list                    # see what's saved for this project
recap search "retry logic"    # find a past decision
recap show 15                 # view one in full
recap edit 15                 # correct it
recap archive 15              # mark it superseded
recap export                  # back up before switching machines
```

Typical flow:

1. `recap init` once, in your project.
2. Work in Claude Code. When you land on a decision worth keeping, `recap save` — review the draft, approve it.
3. Switch to Codex CLI. It asks Recap for relevant context automatically and shows you what it found before using it.
4. Repeat. Stop re-explaining yourself.

### What it won't do

No web chat history (ChatGPT, Claude.ai) — too fragile to scrape reliably. No full transcripts. No auto-trusting AI-generated summaries — everything's a draft until you approve it. No team/cloud sync yet — one developer, one machine, for now.

### Status

Core loop works: save here, retrieve there. Docker vs. embedded Postgres and a couple of concurrency edge cases are still being finalized — see `docs/ROADMAP.md`. Expect rough edges.

### Why "Recap"

A recap is what you'd tell a teammate who missed the meeting — not the transcript, just where you landed and why. That's the goal here, between tools that otherwise share nothing.
