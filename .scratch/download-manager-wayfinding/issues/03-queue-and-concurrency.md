Status: Closed
Labels: wayfinder:grilling
Parent: ../spec.md
Assignee: opencode

## Question

What exact MVP behavior constitutes a reliable queue manager? Define download states, queue ordering and controls, pause/resume/cancel semantics, per-download and global connection limits, retries, failure handling, persistence across restart, and progress reporting. Keep the behavior compatible with the selected aria2 capabilities without prematurely designing non-MVP features.

## Comments

### Resolution

Use a FIFO queue with manual move-up/move-down controls for queued items; active items are not reordered. Run at most three downloads simultaneously by default. Apply one global per-download connection setting, defaulting to four connections per active download, rather than exposing per-item tuning or a separate total-connection budget.

Expose the states **queued**, **active**, **paused**, **failed**, and **complete**. Pausing preserves the partial file and transfer state so resume continues it. Cancelling removes the queue item but preserves partial data for explicit cleanup. Failed items remain visible and retryable; apply bounded automatic retries for transient failures, defaulting to three attempts with backoff, then mark the item failed and continue the queue.

Persist and restore all non-terminal items. After restart, reconcile restored aria2 state with application metadata and automatically continue eligible transfers up to the global active-download cap. Progress reporting is based on aria2's reconciled status and counters; terminal failures remain distinct from completed downloads.
