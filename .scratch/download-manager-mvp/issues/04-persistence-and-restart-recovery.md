# 04 — Persistence and Restart Recovery

**What to build:** Download progress and queue metadata survive normal application restarts, and eligible work resumes safely after an engine failure or unexpected application stop without duplication.

**Blocked by:** 03 — Queue Scheduling and Lifecycle Controls

**Status:** ready-for-agent

- [ ] Non-terminal queue items and required metadata persist periodically within the configured persistence interval.
- [ ] Normal exit saves the session, requests graceful engine shutdown, waits for a bounded period, and force-terminates only when necessary.
- [ ] Startup restores session and control-file state, reconciles engine identifiers and statuses with application metadata, and never silently duplicates an item.
- [ ] Eligible work resumes up to the active-download cap while paused, failed, and complete items retain their expected semantics.
- [ ] Engine readiness failure, unexpected exit, recovery, and concurrent managed-instance prevention are externally observable and tested.
