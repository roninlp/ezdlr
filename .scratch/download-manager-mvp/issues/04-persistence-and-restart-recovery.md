# 04 — Persistence and Restart Recovery

**What to build:** Download progress and queue metadata survive normal application restarts, and eligible work resumes safely after an engine failure or unexpected application stop without duplication.

**Blocked by:** 03 — Queue Scheduling and Lifecycle Controls

**Status:** completed

- [x] Non-terminal queue items and required metadata persist periodically within the configured persistence interval.
- [x] Normal exit saves application metadata before graceful engine shutdown, which waits for a bounded period and force-terminates only when necessary.
- [x] Startup restores application state, reconciles engine identifiers and statuses with application metadata, and avoids duplicate recovered transfers.
- [x] Eligible work resumes up to the active-download cap while paused, failed, and complete items retain their expected semantics.
- [x] Engine readiness failure, unexpected exit, recovery, and concurrent managed-instance prevention are externally observable and tested.

## Comments

### Implementation

Implemented atomic private state persistence, startup reconciliation, process supervision, and restart-focused tests.

- Added `state.json` persistence for queue metadata, configuration, IDs, and retry deadlines.
- Added aria2 recovery enumeration across active, waiting, and stopped transfers.
- Wired restore and periodic persistence into application startup and shutdown.
- Added single-child exit monitoring, readiness failure reporting, active-item failure handling after an unexpected exit, and profile-lock protection tests.

Validation: `go test -race ./...`, `go vet ./...`, and `npm run build` from `frontend/` passed.
