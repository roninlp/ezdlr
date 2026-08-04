# Architecture Deepening

Status: Completed
Labels: completed

## Problem Statement

The MVP download manager works end-to-end, but its internal architecture has begun to leak. The download engine is represented by six one-method capability interfaces that the service discovers through type-assertions at nearly every call site, forcing test doubles into embed-ladder patterns across three files. The aria2 adapter grows three unrelated responsibilities — process supervision, RPC serialization, and response mapping — inside one module, leaving lock acquisition, port selection, readiness polling, and bounded shutdown exercisable only by tests that require a real aria2 binary. A read-named `Snapshot()` method is, in practice, the application's reconcile-schedule-retry-persist loop, so tests drive queue scheduling by reading. The item-creation recipe exists twice, once for single URL intake and once for clipboard batch confirmation, and the pure review function that was extracted for tests has a mutating twin that re-validates inside the service mutex. Finally, the Wails `App` type mirrors eleven of the service's methods verbatim while holding the one real responsibility — reading the clipboard through the Wails runtime context — in a single one of its twelve methods.

Future changes to any of these areas touch scattered, duplicated, or hardness-named code, and the tests that should pin behavior instead encode it through side-effects of the wrong verbs.

## Solution

Deepen five modules across the codebase so that each carries a single responsibility behind a narrow interface, the application's engine seam collapses to one deep contract, the aria2 adapter splits into a supervised-process seam and a pure RPC adapter, the engine's reconcile-schedule-persist loop becomes an explicit background module rather than a side-effect of a read, clipboard intake shares one enqueue recipe, and the Wails `App` stops mirroring the service's interface and owns the runtime adapter it is actually responsible for.

The work preserves the existing app-facing download-service boundary as the primary test seam and introduces one new local seam behind the engine adapter for process supervision. No user-visible behavior changes; the entire scope is observable as a refactor that keeps the existing test suite green (adjusted to the deepened interfaces) and adds tests for the previously-untestable process-supervision paths.

## User Stories

1. As a developer, I want the download engine to be represented by one deep interface rather than six capability interfaces, so that a behavior change is a compile error instead of a new type-assertion at every call site.
2. As a developer, I want the fake engine to implement one interface directly, so that test doubles stop embedding ladders of partial interface implementations.
3. As a developer, I want the service to call the engine through a single interface, so that the engine contract lives in one place and capability drift is caught by the compiler.
4. As a developer, I want process supervision — binary launch, port allocation, secret generation, lock acquisition, readiness polling, exit handling, bounded shutdown — to live behind its own seam, so that I can test it with a fake exit channel instead of a real aria2 binary.
5. As a developer, I want the RPC adapter half of the engine to hold only the JSON-RPC client and the GID map, so that aria2 RPC serialization and response mapping are testable without process lifecycle.
6. As a developer, I want the real-binary adapter contract suite to continue verifying RPC serialization, readiness, session save/restore, GID mapping, process shutdown, and loopback/secret configuration, so that the external engine contract stays pinned.
7. As a developer, I want the engine's reconcile-schedule-retry-persist loop to be an explicit background module with its own interface, so that scheduling is tested by ticking the loop rather than by calling a read.
8. As a developer, I want `Snapshot()` to return the queue without side-effects, so that reads are pure and the frontend's 1 Hz poll has no hidden write behavior.
9. As a developer, I want mutating operations to signal dirty instead of spawning scattered `go saveState()` goroutines, so that the persistence policy lives in one place.
10. As a developer, I want the engine loop to own the existing 30-second save ticker, so that periodic persistence is one heartbeat inside one module.
11. As a developer, I want single-URL intake and clipboard batch confirmation to share one enqueue recipe, so that the item-creation logic — ID formatting, timestamp, engine add, GID lookup, next-ID increment, append, schedule, persist — lives in one place.
12. As a developer, I want the pure clipboard review function to remain the test surface for intake classification, so that extract, validate, and dedupe behavior stays deterministic and mutex-free.
13. As a developer, I want `ConfirmClipboard` to become a thin loop over the accepted review that delegates to the shared enqueue helper, so that batch failures share the single-URL path.
14. As a developer, I want the Wails `App` to stop mirroring the service's method set, so that the binding surface is the service's interface rather than a pass-through copy of it.
15. As a developer, I want the clipboard read responsibility — which needs the Wails startup context — to live in a named runtime adapter, so that the one real `App` responsibility is explicit and localizable.
16. As a developer, I want the engine adapter to satisfy the single deep interface from the supervised process it holds, so that the adapter composes two seams rather than inheriting three responsibilities.
17. As a developer, I want the queue scheduling tests to drive the loop directly, so that FIFO ordering, active-cap enforcement, automatic progression, bounded retries, and terminal-state transitions are observable through one tick instead of through `Snapshot()`.
18. As a developer, I want restart-recovery reconciliation to run inside the engine loop, so that `Restore()` restores persisted state and hands the engine's recovery snapshot to the loop rather than running reconciliation inline.
19. As a developer, I want unexpected-engine-exit handling to live in the engine loop, so that marking active items failed and requesting a save is one event in one module instead of a separate goroutine watching an exit channel.
20. As a developer, I want the engine loop's save policy to batch dirty signals, so that a burst of intake or lifecycle operations produces one persist rather than nine.
21. As a developer, I want the architecture deepening to keep the existing service-level test suite green after adjusting the fakes to the single deep engine interface, so that the refactor is verifiably behavior-preserving.
22. As a developer, I want process-supervision unit tests to cover lock contention, port allocation, secret generation, readiness timeout, exit-before-readiness, and bounded shutdown, so that the lifecycle paths that previously required a real binary are now deterministic.
23. As a developer, I want the three existing persisting and recovery tests — round-trip, reconciliation without duplication, and active-items-failed-after-exit — to continue passing against the deepened engine loop, so that persistence semantics are preserved.
24. As a developer, I want the frontend to continue polling `Snapshot()` at 1 Hz with no awareness of the engine loop, so that the UI contract is unchanged.
25. As a developer, I want the removal of the eleven `App` pass-through methods to be transparent to the TypeScript frontend, so that the Wails-generated bindings remain method-compatible.
26. As a maintainer, I want the deletion test to pass for every module I delete in this work, so that I am confident each refactor concentrates complexity rather than moving it.
27. As a future explorer, I want the engine seam to be the single contract documented in the spec, so that navigating the codebase does not require discovering six capability interfaces.
28. As a future explorer, I want a context glossary entry for the supervised-process concept, so that the term is available before implementation names a module after it.
29. As a future explorer, I want the engine loop concept named in the context glossary, so that the reconcile-schedule-persist responsibility has a stable name.
30. As a future explorer, I want any decision that rejects one of these candidates to be recorded as an ADR, so that a future architecture review does not re-suggest it.

## Implementation Decisions

- Collapse the six engine capability interfaces (`DownloadEngine`, `DownloadStatusProvider`, `DownloadRecoveryEngine`, `DownloadEngineExitProvider`, `DownloadLifecycleEngine`, `DownloadGIDProvider`) into one deep `DownloadEngine` interface on the existing engine seam. The deepened interface carries add, status, pause, resume, cancel, recover, exited, GID, and shutdown.
- The `DownloadService` stops type-asserting the engine at each call. It calls the single interface; the engine adapter implements all of it.
- The existing `FakeEngine` grows the remaining methods with trivial canned returns (status returns `EngineStatus{}`, lifecycle methods return nil, recover returns an empty slice, exited returns nil channel). The embed-ladder fakes in the test files (`queueFakeEngine`, `recoveryFakeEngine`, `exitFakeEngine`) collapse into direct implementations of the single interface.
- Split the `Aria2Engine` module along two seams. Introduce a `SupervisedProcess` module behind its own interface: binary path, data directory, download directory, port allocation, secret generation, config-file materialization, lock-file ownership, readiness polling, exit-channel publication, and bounded shutdown. The existing `Aria2Engine` becomes the RPC adapter that holds one `SupervisedProcess` and exposes the deep `DownloadEngine` surface.
- The `SupervisedProcess` seam admits two adapters: the real aria2 child process in production and a fake exit-channel-backed stub in tests, so the seam is justified by two concrete implementations.
- Lock acquisition, port selection, secret generation, config materialization, and readiness timeout move from `NewManagedAria2` into the `SupervisedProcess` constructor and become unit-testable with a fake binary (e.g. `/bin/false` for exit-before-readiness) and a stub for the happy path.
- The dual `Shutdown` paths in `Aria2Engine` — wait on `processDone` vs context-kill on timeout — move into `SupervisedProcess.Shutdown()`; the RPC adapter's shutdown composes `aria2.saveSession` + `aria2.shutdown` with the supervised process's bounded stop.
- Deepen the engine loop into an explicit background module. The loop owns the existing 30-second save ticker, the reconcile pass against engine recovery, the schedule pass (active-cap enforcement, FIFO progression, bounded retry with backoff), the unexpected-exit handler, and the dirty-signal flush. `Snapshot()` becomes a pure read; mutating operations (`AddURL`, `Pause`, `Resume`, `Cancel`, `Retry`, `ConfirmClipboard`) signal dirty instead of spawning `go saveState()`.
- The engine loop exposes one interface for testing: the loop's tick (or an equivalent test-only advance). Queue scheduling tests drive the tick instead of calling `Snapshot()`; the existing assertions on FIFO order, active-cap enforcement, automatic progression, bounded retries, and terminal-state semantics move to the tick-driven harness.
- `Restore()` reads persisted state, restores items and configuration into the service, and hands the engine's `Recover()` snapshot to the engine loop's first tick rather than running reconciliation inline under the service mutex.
- Unexpected-engine-exit handling — currently a separate goroutine watching `Exited()` and marking active items failed — moves into the engine loop as one event handled in one module.
- Pull one private `enqueueLocked(url)` helper out of `AddURL` and `ConfirmClipboard`. The helper owns ID formatting, `AddedAt` stamping, `engine.Add`, GID lookup, `nextID` increment, item append, and a dirty signal. Both call sites delegate to it.
- The pure `reviewClipboardText` function remains the test surface for intake classification — extract, validate, dedupe — and stays mutex-free. `ConfirmClipboard` becomes a thin loop: re-validate against the current queue for each accepted entry, call the shared enqueue helper, classify enqueue failures.
- `CancelClipboardReview` stays a no-op value-object discard; the explicit call remains at the app boundary so cancellation is observable.
- Bind `DownloadService` to Wails directly for queue, lifecycle, persistence, and configuration methods. Introduce a `ClipboardReader` runtime adapter that holds the Wails startup context and exposes `ReadClipboard`, `ReviewClipboard`, and `ConfirmClipboard`. The existing `App` type either disappears or shrinks to compose the service and the clipboard reader.
- The eleven one-line forwarder methods in `App` are deleted; their bindings continue to resolve against the service directly so the frontend's generated method names are preserved.
- The clipboard-runtime seam is local-substitutable: production uses the Wails runtime adapter; a test adapter returns canned clipboard text so the existing clipboard intake tests do not depend on a windowing context.
- No persisted-state schema changes. The `persistedState` version stays at 1; retry deadlines, items, configuration, and next-ID migrate to the engine loop unchanged.
- No ADRs currently exist for any touched area. The wayfinding spec named the Wails/Go, local-aria2, and queue-policy decisions as the architectural baseline; this deepening preserves those decisions and does not introduce a new framework, a new transport, or a new queue policy.

## Testing Decisions

- The primary test seam remains the app-facing download-service boundary (`DownloadService`'s public methods) exercised through a controllable fake engine adapter — the highest existing seam, preferred over any new one. The deepening keeps this seam and does not add a parallel one.
- One new local seam is introduced behind the engine adapter: the `SupervisedProcess` interface. Its tests use a fake exit-channel-backed adapter so that lock acquisition, port allocation, secret generation, readiness timeout, exit-before-readiness, and bounded shutdown are deterministic and do not require a real aria2 binary.
- The existing real-binary adapter contract suite stays as the integration test for the RPC adapter half — RPC serialization, readiness, session save/restore, GID mapping, process shutdown, loopback/secret configuration, and concurrent-instance lock prevention. These tests continue to skip when `/usr/bin/aria2c` is absent.
- The queue scheduling tests (`TestQueueSchedulesThreeAndProgressesFIFO`, `TestQueueLifecycleControlsPreserveItemSemantics`, `TestQueuedItemsMoveWithoutReorderingActiveItems`, `TestQueueRetriesTransientFailuresAndStopsAtLimit`, `TestFailedDownloadDoesNotBlockLaterWorkAndCanBeRetried`) are rewritten to drive the engine loop's tick instead of calling `Snapshot()`. The observable outcomes — FIFO order, active-cap enforcement, automatic progression, bounded retries, terminal-state transitions — are unchanged; the verb that produces them changes from a read to a tick.
- The persistence and recovery tests (`TestJSONStateStoreRoundTripProtectsStateFile`, `TestRestoreReconcilesEngineStateWithoutDuplicatingTransfers`, `TestServiceMarksActiveItemsFailedAfterUnexpectedEngineExit`) continue to pass against the deepened engine loop. `Restore()` hands the recovery snapshot to the loop's first tick; the unexpected-exit test drives the loop tick after publishing on the fake exit channel.
- The clipboard intake tests (`TestReviewClipboardExtractsAndClassifiesURLsWithoutEnqueueing`, `TestConfirmClipboardUsesConfiguredDestinationAndReturnsItemLinks`, `TestConfirmClipboardContinuesAfterPartialEnqueueFailure`, `TestClipboardReviewCancellationDoesNotEnqueue`) continue to assert through `ReviewClipboard` and `ConfirmClipboard`. The pure `reviewClipboardText` helper remains directly testable without the service mutex.
- The `AddURL` tests (`TestAddURLCreatesQueuedItemAndUsesConfiguredDestination`, `TestAddURLRejectsUnsupportedAndCredentialBearingURLs`, `TestAddURLRejectsDuplicatesWithoutCallingEngine`) continue to pass against the shared enqueue helper; no behavior change is observable through the service boundary.
- A good test asserts external behavior through the highest seam, uses stable domain states and service results, avoids engine internals and frontend implementation details, and stays deterministic except for the explicitly scoped real-binary adapter contract suite. The deepening widens the deterministic surface — process supervision moves from binary-gated to fake-driven.
- Prior art: `service_test.go` (fake-engine service tests), `persistence_test.go` (recovery fake + exit fake), `aria2_test.go` (real-binary contract + binary-absent skips). These files establish the convention of one fake per adapter and observable-outcome assertions; the deepening follows that convention with a consolidated fake for the single deep `DownloadEngine` interface and a new fake for `SupervisedProcess`.
- The frontend is not retested as part of this work. Its 1 Hz `Snapshot()` poll and the generated Wails bindings remain method-compatible; `react-doctor` is the appropriate later check if any UI regression surfaces.

## Out of Scope

- User-visible behavior changes. No new features, no new states, no new UI flows, no new configuration knobs, no schema migration.
- Renaming the Wails-generated frontend bindings, the TypeScript method names, the JSON field names, or the persisted-state version.
- Changing the download protocol scope. Still direct HTTP and HTTPS only.
- Changing the queue policy. Still FIFO, manual move-up/down for queued items only, active-cap of three, four connections per active download, three bounded automatic retries with backoff.
- Changing the persistence format, the save cadence semantics (no more than the configured interval of queue changes lost), or the restart-recovery contract (eligible work resumes without duplication, terminal semantics preserved).
- Changing the managed-engine policy of one local aria2 child process per user profile, loopback-only RPC, per-process secret, HTTPS certificate verification, dedicated per-user data directory, and bundled pinned binary.
- Replacing Wails with Tauri or any other framework. The wayfinding decision stays.
- Adding WebSocket-based aria2 notifications, an in-app updater, file associations, macOS support, or BitTorrent/Metalink/FTP/authenticated downloads.
- Per-download destination selection, per-download connection tuning, a separate global total-connection budget, background clipboard monitoring, or automatic deletion of partial files after cancellation.
- Rewriting the frontend. The SolidJS UI continues to poll `Snapshot()` at 1 Hz; the deepening is invisible to it.
- Profiling or performance validation work. Performance budgets belong to the existing MVP performance issue, not this architectural pass.

## Further Notes

- This spec is informed by an architecture review that surfaced five deepening candidates. The review's top recommendation is Candidate 01 — collapsing the six capability interfaces — because it touches every call site in the download service, deletes the embed-ladder fake pattern across three test files, and unblocks the engine-loop and adapter-split work that follows. Implementing the candidates in roughly the review's order (01 → 02 → 03 → 04 → 05) minimizes churn: each candidate becomes easier after the seam above it deepens. This ordering is a recommendation, not a constraint — the candidates are independently shippable.
- No `CONTEXT.md` or ADRs exist in this repo yet. The domain glossary should be created lazily, when the first term that needs a stable name appears during implementation. Two terms this work is likely to introduce — the supervised-process concept and the engine-loop concept — are candidates for the glossary when they crystallize. If a reviewer rejects any candidate with a load-bearing reason, that reason should be recorded as an ADR so future architecture reviews do not re-suggest the same thing.
- The architecture review produced a visual report of the five candidates with before/after diagrams; that report lives outside the repo in the OS temp directory and is not a source of truth once this spec is published.
- The single-seam principle is preserved: one primary test seam (the download-service boundary + fake engine) plus one local seam behind the engine adapter (the supervised process). No new public seams are introduced anywhere else in the codebase.

## Implementation Notes

- All five architecture-deepening issues are complete. The implementation spans commits `fc3e858` through `c0a8eb9`; the issue files record each ticket's completion state and verification.
