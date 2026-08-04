# 04 — Share one enqueue recipe between AddURL and ConfirmClipboard

**What to build:** Single-URL intake and clipboard batch confirmation stop carrying two copies of the item-creation recipe. One private `enqueueLocked(url)` helper owns ID formatting, `AddedAt` stamping, `engine.Add`, GID lookup, `nextID` increment, item append, and the dirty signal — and both `AddURL` and `ConfirmClipboard` delegate to it. `ConfirmClipboard` becomes a thin loop: re-validate each accepted entry against the current queue for that entry, call the shared enqueue helper, and classify enqueue failures so a batch failure shares the single-URL path. The pure `reviewClipboardText` function remains the test surface for intake classification (extract, validate, dedupe) and stays mutex-free; the test for cancellation continues to assert that a cancelled review enqueues nothing. From the user's perspective nothing changes; the refactor is verifiable by the item-creation block existing exactly once and the existing AddURL and clipboard-intake tests passing unchanged through the service boundary.

**Blocked by:** 03 — Deepen the reconcile/schedule/persist loop into an explicit engine-loop background module. The helper signals dirty (the convention introduced by 03) rather than spawning `go saveState()`, and the work shares `service.go`/`clipboard.go` with 03.

**Status:** completed

- [x] A private `enqueueLocked(url)` helper exists and owns ID formatting, `AddedAt` stamping, `engine.Add`, GID lookup, `nextID` increment, item append, and the dirty signal.
- [x] `AddURL` delegates to the shared helper; the inline item-creation block is gone.
- [x] `ConfirmClipboard` is a thin loop over the accepted review: per-entry re-validation against the current queue, enqueue via the shared helper, and enqueue-failure classification; the inline item-creation block is gone.
- [x] The pure `reviewClipboardText` function remains the mutex-free test surface for intake classification and continues to be directly testable without the service mutex.
- [x] The item-creation recipe exists exactly once in the codebase.
- [x] `TestAddURLCreatesQueuedItemAndUsesConfiguredDestination`, `TestAddURLRejectsUnsupportedAndCredentialBearingURLs`, `TestAddURLRejectsDuplicatesWithoutCallingEngine`, `TestReviewClipboardExtractsAndClassifiesURLsWithoutEnqueueing`, `TestConfirmClipboardUsesConfiguredDestinationAndReturnsItemLinks`, `TestConfirmClipboardContinuesAfterPartialEnqueueFailure`, and `TestClipboardReviewCancellationDoesNotEnqueue` continue to pass unchanged through the service boundary.
- [x] `go test ./...` stays green; no user-visible behavior change.

## Comments

- Implemented the shared `enqueueLocked` recipe for single-URL and clipboard intake.
- Verified focused intake tests, `go vet ./...`, and `go test ./...`.
