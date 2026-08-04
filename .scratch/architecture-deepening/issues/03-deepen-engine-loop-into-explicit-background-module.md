# 03 — Deepen the reconcile/schedule/persist loop into an explicit engine-loop background module

**What to build:** The engine's reconcile-schedule-retry-persist loop becomes an explicit background module with its own interface rather than a side-effect of a read. `Snapshot()` returns the queue without side-effects; the 30-second save ticker, the reconcile pass against engine recovery, the schedule pass (active-cap enforcement, FIFO progression, bounded retry with backoff), the unexpected-exit handler, and the dirty-signal flush all live inside one engine loop. Mutating operations signal dirty instead of spawning scattered `go saveState()` goroutines, so the persistence policy lives in one place; periodic persistence is one heartbeat inside one module. `Restore()` reads persisted state, restores items and configuration into the service, and hands the engine's `Recover()` snapshot to the loop's first tick rather than running reconciliation inline under the service mutex. Unexpected-engine-exit handling moves into the loop as one event handled in one module instead of a separate goroutine watching an exit channel. The loop batches dirty signals so that a burst of intake or lifecycle operations produces one persist rather than nine. The loop exposes one tick interface for testing: queue scheduling tests drive the tick instead of calling `Snapshot()`. The frontend continues polling `Snapshot()` at 1 Hz with no awareness of the loop and no hidden write behavior on reads.

**Blocked by:** 01 — Collapse six engine capability interfaces into one deep DownloadEngine interface. The loop drives scheduling through the consolidated fake and the single deep interface, and the engine's `Exited()`/`Recover()` are now part of that one interface.

**Status:** complete

- [x] An engine-loop background module owns the 30-second save ticker, the reconcile pass against `Recover()`, the schedule pass (FIFO/active-cap/retry-backoff), the unexpected-exit handler, and the dirty-signal flush.
- [x] `Snapshot()` is a pure read: no live status poll, no `scheduleLocked()`, no `go saveState()`.
- [x] `AddURL`, `Pause`, `Resume`, `Cancel`, `Retry`, and `ConfirmClipboard` signal dirty instead of spawning `go saveState()`; the only synchronous save calls are the loop's ticker heartbeat and `Shutdown()`.
- [x] `Restore()` restores persisted items and configuration and hands the engine's `Recover()` snapshot to the loop's first tick; `reconcile()` no longer runs inline under the service mutex.
- [x] Unexpected-engine-exit handling lives in the loop as one event; the separate observer goroutine and its `handleEngineExit` mark-active-failed logic move into the loop.
- [x] The loop's save policy batches dirty signals so a burst of operations yields one persist, not nine.
- [x] The loop exposes a tick interface for tests.
- [x] The five scheduling tests (`TestQueueSchedulesThreeAndProgressesFIFO`, `TestQueueLifecycleControlsPreserveItemSemantics`, `TestQueuedItemsMoveWithoutReorderingActiveItems`, `TestQueueRetriesTransientFailuresAndStopsAtLimit`, `TestFailedDownloadDoesNotBlockLaterWorkAndCanBeRetried`) drive the tick instead of `Snapshot()`; observable outcomes (FIFO order, active-cap enforcement, automatic progression, bounded retries, terminal-state transitions) are unchanged.
- [x] The three persistence/recovery tests (`TestJSONStateStoreRoundTripProtectsStateFile`, `TestRestoreReconcilesEngineStateWithoutDuplicatingTransfers`, `TestServiceMarksActiveItemsFailedAfterUnexpectedEngineExit`) continue to pass; the exit test publishes on the fake exit channel and then ticks.
- [x] The frontend's 1 Hz `Snapshot()` poll is unchanged; no UI regression.
- [x] `go test ./...` stays green; no user-visible behavior change.
