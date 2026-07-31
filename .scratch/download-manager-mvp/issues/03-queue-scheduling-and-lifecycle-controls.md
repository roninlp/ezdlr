# 03 — Queue Scheduling and Lifecycle Controls

**What to build:** Users can manage a FIFO queue and control individual downloads while the service automatically schedules work within the MVP concurrency and connection policies.

**Blocked by:** 02 — Managed aria2 Single-Download Flow

**Status:** ready-for-agent

- [ ] Queued items display in FIFO order and can be moved up or down without reordering active items.
- [ ] The service starts eligible work automatically, limits active downloads to three by default, and applies four connections per active download.
- [ ] Users can pause, resume, and cancel downloads; pause and cancel preserve partial data, while cancel removes only the queue item.
- [ ] Transient failures retry automatically with backoff up to three attempts, then remain visible as failed without blocking later work.
- [ ] Users can manually retry failed items, and queued, active, paused, failed, and complete states remain distinct.
- [ ] Fake-engine tests cover scheduling, lifecycle transitions, retry limits, progression, and observable status outcomes.
