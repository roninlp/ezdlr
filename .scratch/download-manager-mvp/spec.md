# Minimal Cross-Platform Download Manager MVP

Status: Open
Labels: ready-for-agent

## Problem Statement

People who download files from direct HTTP and HTTPS links need a small desktop queue manager rather than a collection of shell commands or a browser-only workflow. The product must be useful on Arch Linux first and have a credible Windows distribution path, while staying fast and resource-conscious. Users currently lack a reliable way to review batches of links, control concurrent downloads, recover after an application restart, and understand whether a transfer is queued, active, paused, failed, or complete.

## Solution

Build a minimal desktop download manager with Wails v2, a Go application backend, and a TypeScript UI. The application owns one local, bundled aria2 process per user profile and exposes a narrow download-service boundary to the UI. Users can add individual links or review a batch extracted from clipboard text, choose the configured download destination, manage a FIFO queue, pause and resume transfers, retry failures, and recover eligible work after restarting the application.

The MVP supports direct HTTP and HTTPS downloads only. Arch Linux is the first release target, distributed through an AUR-compatible native package; Windows follows through an NSIS installer. Each package includes a pinned, application-compatible aria2 binary and the required launcher, icons, uninstall support, and system-tray entry.

## User Stories

1. As a desktop user, I want to launch a native download manager, so that I can manage downloads without keeping a terminal open.
2. As an Arch Linux user, I want the application to install through an AUR-compatible package, so that it fits my normal system maintenance workflow.
3. As a Windows user, I want an installer that includes all required runtime components, so that I do not need to install Go, aria2, or another dependency first.
4. As a user, I want the application to start directly to a usable queue view, so that I can act without navigating setup screens.
5. As a user, I want to add a valid HTTP or HTTPS URL, so that I can download a direct file link.
6. As a user, I want invalid or unsupported URLs rejected with a clear explanation, so that I know why an item was not added.
7. As a user, I want to invoke clipboard intake explicitly, so that the application never monitors or records clipboard changes in the background.
8. As a user, I want the application to extract every absolute HTTP or HTTPS URL from mixed clipboard text, so that I can process links copied from notes, messages, or pages.
9. As a user, I want surrounding whitespace trimmed from extracted URLs, so that ordinary copied formatting does not prevent intake.
10. As a user, I want malformed and credential-bearing URLs rejected, so that unsafe or unusable links are not silently queued.
11. As a user, I want duplicate links within a clipboard batch detected, so that one pasted link cannot create multiple downloads.
12. As a user, I want links already represented by queued, active, paused, failed, or complete items skipped, so that the queue remains deduplicated.
13. As a user, I want to review accepted links before enqueueing them, so that a batch action never starts downloads unexpectedly.
14. As a user, I want the review step to show rejected and duplicate counts, so that I can assess what will happen before confirming.
15. As a user, I want a confirmed batch to use the configured default download directory, so that all items have predictable destinations.
16. As a user, I want a categorized result after batch intake, so that I can distinguish accepted, duplicate, invalid, and enqueue failures.
17. As a user, I want expandable URL-level details in the batch result, so that I can diagnose a particular link.
18. As a user, I want a link from the batch result to the queued items, so that I can continue managing the downloads immediately.
19. As a user, I want queued items displayed in FIFO order, so that downloads begin predictably.
20. As a user, I want to move queued items up or down manually, so that I can prioritize work without changing active transfers.
21. As a user, I want active items excluded from manual queue reordering, so that an in-progress transfer is not destabilized.
22. As a user, I want to see whether each item is queued, active, paused, failed, or complete, so that I understand its lifecycle.
23. As a user, I want progress and transfer counters based on the actual download engine state, so that displayed progress is trustworthy.
24. As a user, I want at most three downloads active at once by default, so that bandwidth and system resources remain controlled.
25. As a user, I want each active download to use the global default of four connections, so that downloads are reasonably efficient without per-item tuning.
26. As a user, I want a download to start automatically when capacity becomes available, so that the queue continues without manual babysitting.
27. As a user, I want to pause an active download, so that I can temporarily free bandwidth or resources.
28. As a user, I want pausing to preserve partial data and transfer state, so that resuming does not restart the file unnecessarily.
29. As a user, I want to resume a paused download, so that preserved work continues from its partial state.
30. As a user, I want to cancel a download, so that unwanted work leaves the queue.
31. As a user, I want cancellation to preserve partial data for explicit cleanup, so that the application never deletes user data without a separate decision.
32. As a user, I want transient transfer failures retried automatically with backoff, so that temporary network problems do not require intervention.
33. As a user, I want automatic retries bounded at three attempts by default, so that a persistent failure does not block the queue indefinitely.
34. As a user, I want an exhausted item marked failed and kept visible, so that I can inspect and retry it later.
35. As a user, I want to retry a failed item manually, so that I can recover from a temporary problem after the automatic retry budget is exhausted.
36. As a user, I want one failed item not to block later queued items, so that the queue continues making progress.
37. As a user, I want completed items to remain distinguishable from failed items, so that successful and unsuccessful outcomes are not conflated.
38. As a user, I want queue metadata and all non-terminal items persisted, so that application progress survives a normal close.
39. As a user, I want eligible downloads to continue after application restart, so that interruption does not require rebuilding the queue.
40. As a user, I want restored engine state reconciled with application metadata, so that restart recovery does not duplicate or lose an item.
41. As a user, I want a crashed or unexpectedly stopped download engine detected, so that the application can recover rather than showing stale progress forever.
42. As a user, I want the application to restore or restart its managed engine safely, so that ordinary engine failures do not corrupt the queue.
43. As a user, I want only one managed engine instance for my profile, so that two processes cannot compete for the same queue or files.
44. As a user, I want engine communication restricted to loopback and protected by a per-process secret, so that local RPC is not exposed as an unauthenticated network service.
45. As a user, I want session, control, and credential-bearing application data treated as private, so that download state and secrets are not broadly readable.
46. As a user, I want normal application exit to save the session and shut down the engine cleanly, so that the next launch has the most complete state possible.
47. As a user, I want the application to use a deterministic bundled aria2 version, so that behavior does not depend on an unrelated system installation.
48. As a user, I want a launcher, icons, uninstall support, and a system-tray entry, so that the application behaves like a normal desktop program.
49. As a user, I want the application to remain responsive while three downloads are active, so that queue controls and status remain usable under load.
50. As a user, I want low idle CPU and modest idle memory use, so that leaving the manager open has negligible impact on my system.

## Implementation Decisions

- Use Wails v2 with Go for the application and backend, and TypeScript for the UI. Keep download policy in the Go backend rather than duplicating it in frontend code.
- Expose a narrow app-facing download-service interface for intake, queue operations, lifecycle controls, status snapshots, configuration, and recovery. The UI must not communicate with aria2 directly.
- Launch and own exactly one local aria2 child process per user profile. Select an available loopback RPC port, generate a per-process RPC secret, wait for RPC readiness, and supervise the child process.
- Use aria2 JSON-RPC over HTTP at `/jsonrpc` for the MVP. Poll reconciled status through the adapter; WebSocket notifications are deferred and must not be a correctness dependency.
- Make the Go aria2 adapter responsible for RPC serialization, request timeouts, response validation, GID mapping, startup restoration, and crash/restart reconciliation.
- Let the application own user-facing queue policy, item metadata, configuration, and lifecycle state. Let aria2 own transfer execution, partial-file control files, transport retries, progress counters, and GIDs.
- Start aria2 loopback-only with a per-process RPC secret, HTTPS certificate verification, a dedicated per-user data directory, and the configured download directory.
- Persist all non-terminal queue items and restore them using aria2 session input and control files. Reconcile restored GIDs and statuses against application metadata and never silently create a duplicate item.
- Save sessions periodically so an unexpected crash loses no more than the configured persistence interval of queue changes. On normal exit, save the session, request aria2 shutdown, wait for a bounded period, and force-terminate only if required.
- Use the states `queued`, `active`, `paused`, `failed`, and `complete`. Keep terminal failures distinct from completed downloads.
- Use FIFO scheduling with manual move-up and move-down operations for queued items only. Start eligible work automatically up to a default global active-download cap of three.
- Apply one global per-download connection setting, defaulting to four connections per active download. Do not expose per-item connection tuning or a separate total-connection budget in the MVP.
- Preserve partial files on pause and cancel. Cancellation removes the queue item but does not delete partial data; cleanup is an explicit later action and is outside this MVP.
- Apply bounded automatic retries for transient failures, defaulting to three attempts with backoff. Mark exhausted items failed and continue scheduling later items.
- Inspect clipboard content only after explicit user action. Extract absolute HTTP and HTTPS URLs from mixed text, trim surrounding whitespace, reject malformed or credential-bearing URLs, deduplicate within the batch, and skip URLs represented by any existing queue item.
- Require review and confirmation before clipboard items are enqueued. Use the configured default destination for the whole batch and return categorized per-URL results for accepted, duplicate, invalid or rejected, and failed-to-enqueue outcomes.
- Bundle and pin an application-compatible aria2 binary for each supported platform. Do not require a system-installed aria2 binary for the MVP.
- Package Arch Linux first as a native AUR-compatible PKGBUILD and package Windows next with a Wails-default NSIS installer. Defer an in-app updater and file associations.
- Provide desktop launcher integration, icons, uninstall support, and a system-tray entry in both platform packages. Keep bundled aria2 private to the application and invoke it deterministically.
- Keep the initial protocol scope to direct HTTP and HTTPS downloads. Do not add authentication, browser interception, browser extensions, BitTorrent, Metalink, FTP, scheduling, or macOS support.
- Treat the existing wayfinding decisions as the architectural baseline. No ADR currently exists for this area; any implementation decision that changes the selected Wails/Go, local aria2, or queue policy should be recorded before implementation proceeds.

## Testing Decisions

- Use the highest existing seam: the app-facing download-service boundary. Exercise user-visible intake, queue policy, lifecycle controls, persistence, recovery, and status behavior through that interface rather than testing UI internals or aria2 implementation details.
- Use a controllable fake aria2 adapter at that boundary for deterministic tests of queue scheduling, state transitions, retry limits, pause/resume/cancel behavior, duplicate detection, restart reconciliation, and engine failures.
- Add a small adapter contract suite against a real local aria2 process for RPC serialization, readiness, session save/restore, GID mapping, process shutdown, and loopback/secret configuration. These tests verify the external engine contract without making every queue test process-dependent.
- Test clipboard intake with mixed text, whitespace, malformed links, unsupported schemes, credential-bearing links, intra-batch duplicates, and duplicates against every non-terminal and terminal application state.
- Test batch review and confirmation separately from enqueue execution, including cancellation of review, configured destination propagation, categorized results, partial enqueue failure, and no background clipboard reads.
- Test queue behavior through observable outcomes: FIFO ordering, manual queued-item movement, active-cap enforcement, automatic progression after completion or failure, bounded retries with backoff, and distinct terminal states.
- Test persistence and restart by creating representative queue state, restarting the service, reconciling fake engine state, and asserting that eligible work resumes without duplication while paused, failed, and complete items retain their expected semantics.
- Test process supervision through externally visible service behavior: readiness failure, unexpected child exit, bounded shutdown, recovery, and prevention of concurrent managed instances.
- Test the TypeScript UI at the highest available user-facing boundary using existing project UI test conventions once the UI exists. Assert rendered state and user interactions, not component structure, hook usage, or CSS implementation.
- Validate the three representative performance workflows: launch to usable UI, clipboard batch review, and three concurrent downloads including pause, resume, and restart. Use app timestamps and lightweight OS process measurements on a modest Arch Linux laptop, then repeat on Windows when supported.
- A good test asserts external behavior, uses stable domain states and service results, avoids aria2 internals and frontend implementation details, and remains deterministic except for the explicitly scoped real-process contract and performance checks.
- There is no existing application test prior art because the repository currently contains the wayfinding map and research assets only. Establish the download-service contract and fake-adapter tests as the initial testing pattern, then follow the repository's conventions as implementation adds them.

## Out of Scope

- Browser extensions, browser interception, and automatic browser link capture.
- BitTorrent, Metalink, FTP, authenticated downloads, scheduling, and macOS support.
- File associations, an in-app auto-updater, and requiring or discovering a system-installed aria2.
- Per-download destination selection, per-download connection tuning, and a separate global total-connection budget.
- Background clipboard monitoring.
- Automatic deletion of partial files after cancellation.
- WebSocket-based aria2 notifications, a dedicated profiling system, or hard hardware-specific performance budgets.
- Implementing the application as part of this specification publication; implementation is the work that follows this spec.

## Further Notes

- The repository has no application source yet, so this spec defines the initial product and service boundary rather than prescribing changes to existing modules.
- The primary release order is Arch Linux first and Windows next. Wails remains the selected framework unless measured MVP validation shows it misses the responsiveness or resource goals; Tauri is the documented fallback, not part of the initial implementation.
- The configured destination and retry/connection/concurrency defaults should be user-visible configuration values only where the MVP stories require them; avoid adding settings screens or policy knobs without a concrete user-facing need.
