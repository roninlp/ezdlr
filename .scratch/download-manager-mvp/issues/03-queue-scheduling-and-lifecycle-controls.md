# 03 — Queue Scheduling and Lifecycle Controls

**What to build:** Users can manage a FIFO queue and control individual downloads while the service automatically schedules work within the MVP concurrency and connection policies.

**Blocked by:** 02 — Managed aria2 Single-Download Flow

**Status:** completed

- [x] Queued items display in FIFO order and can be moved up or down without reordering active items.
- [x] The service starts eligible work automatically, limits active downloads to three by default, and applies four connections per active download.
- [x] Users can pause, resume, and cancel downloads; pause and cancel preserve partial data, while cancel removes only the queue item.
- [x] Transient failures retry automatically with backoff up to three attempts, then remain visible as failed without blocking later work.
- [x] Users can manually retry failed items, and queued, active, paused, failed, and complete states remain distinct.
- [x] Fake-engine tests cover scheduling, lifecycle transitions, retry limits, progression, and observable status outcomes.

## Comments

### Implementation

Implemented in commit `47c3bf7`.

- Added FIFO scheduling with a default active limit of three and four aria2 connections per download.
- Added queued-item movement, pause/resume/cancel controls, bounded automatic retries with backoff, and manual failed-item retry.
- Exposed queue and lifecycle controls through the app-facing Wails methods.
- Added fake-engine coverage for scheduling, progression, ordering, lifecycle controls, retry limits, and failed-item recovery.

Validation: `go test ./...`, `go vet ./...`, and `npm run build` from `frontend/` passed.
