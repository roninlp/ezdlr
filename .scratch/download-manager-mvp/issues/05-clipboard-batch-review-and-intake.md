# 05 — Clipboard Batch Review and Intake

**What to build:** After an explicit user action, the application extracts and reviews direct HTTP/HTTPS links from clipboard text before enqueueing them, then reports exactly what happened for every link.

**Blocked by:** 03 — Queue Scheduling and Lifecycle Controls

**Status:** ready-for-agent

- [ ] Clipboard content is read only after an explicit user action and is never monitored in the background.
- [ ] Mixed text yields absolute HTTP and HTTPS URLs with surrounding whitespace trimmed; malformed, unsupported, and credential-bearing URLs are rejected clearly.
- [ ] Intra-batch duplicates and URLs represented by queued, active, paused, failed, or complete items are skipped and counted.
- [ ] A review shows accepted URLs and rejected or duplicate counts, and cancellation prevents enqueueing.
- [ ] Confirmation enqueues accepted URLs to the configured default destination without starting unexpected duplicate work.
- [ ] Results categorize accepted, duplicate, invalid or rejected, and enqueue-failure outcomes, expose URL-level details, and link back to queued items.
- [ ] Service and UI tests cover parsing, review, confirmation, cancellation, configured destination propagation, partial enqueue failure, and no background reads.
