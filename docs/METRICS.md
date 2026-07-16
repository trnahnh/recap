# Success Metrics — Recap

## Target metrics

| Metric | Target | Status / caveat |
|---|---|---|
| Install time | Under 5 minutes | Conditional on Docker already installed, until ADR-002 resolved |
| Cross-tool handoff | A decision saved from one tool loads in another | Core value prop — must be validated end-to-end, not just unit tested |
| Repeated-context reduction | Developer doesn't re-explain known decisions | Qualitative — needs real usage, not just a demo |
| Search relevance | Results usually match the current task | See eval set below — currently no regression check exists |
| Retrieval latency | Under 1 second | Measured through the daemon's connection pool, not a cold-start connection. No query plan validated yet at scale (10k+ records) |
| Correction ability | Developer can edit/delete inaccurate records | Straightforward CRUD, low risk |
| Context size | Stays within a configurable limit | No concrete default number set yet — Fix-before-v1 |
| Network isolation | No external calls required for normal use | Docker image pull happens once at install, not runtime — confirm this holds with embedded Postgres path too |

## Retrieval quality eval (missing — needs to be built)

There is currently no way to know if search/ranking is actually good
before shipping. Before Phase 4 is considered done:

1. Build a small fixed set of `(task description) → (expected relevant
   record IDs)` pairs against a sample project.
2. Run this set as a regression check any time the ranking logic changes.
3. Track precision/recall informally — doesn't need to be rigorous, just
   needs to exist so "search results are usually related to the task"
   isn't purely a vibe check.

## What's not yet measurable

- Concurrency correctness (two tools writing simultaneously) has no
  defined test scenario yet — needs one before Phase 1b is considered
  complete.
- Secret-filtering false-negative rate is unknown; regex-based detection
  has no measured miss rate against a realistic secret sample set.
