# 05 — Bind DownloadService to Wails directly; introduce a ClipboardReader runtime adapter

**What to build:** The Wails `App` stops mirroring the download service's method set. The eleven one-line pass-through methods are deleted; `DownloadService` binds to Wails directly for queue, lifecycle, persistence, and configuration methods, and the Wails-generated TypeScript method names stay method-compatible so the frontend is transparent to the change. The one real `App` responsibility — reading the clipboard through the Wails startup context — moves into a named `ClipboardReader` runtime adapter that holds the Wails startup context and exposes `ReadClipboard`, `ReviewClipboard`, and `ConfirmClipboard`. The clipboard-runtime seam is local-substitutable: production uses the Wails runtime adapter and tests use a canned-clipboard-text adapter so the existing clipboard intake tests do not depend on a windowing context. `CancelClipboardReview` stays a no-op value-object discard but remains an explicit call at the app boundary so cancellation is observable. The `App` type either disappears or shrinks to compose the service and the clipboard reader. From the user's perspective nothing changes; the refactor is verifiable by the absence of one-line service forwarders on `App`, method-compatible frontend bindings, and passing clipboard intake tests through the test adapter.

**Blocked by:** None — can start immediately. This work touches `App`/`main` binding wiring plus a new runtime adapter file and is independent of the engine-interface, adapter-split, engine-loop, and enqueue-recipe work.

**Status:** completed

- [x] The eleven one-line forwarder methods on `App` are deleted; no pass-through to `DownloadService` remains on `App`.
- [x] `DownloadService` binds to Wails directly for queue, lifecycle, persistence, and configuration methods.
- [x] A `ClipboardReader` runtime adapter holds the Wails startup context and exposes `ReadClipboard`, `ReviewClipboard`, and `ConfirmClipboard`.
- [x] Production uses the Wails `ClipboardReader` adapter; tests use a canned-clipboard-text adapter so clipboard intake tests do not require a windowing context.
- [x] `CancelClipboardReview` remains an explicit, observable no-op at the app boundary.
- [x] The `App` type either disappears or shrinks to compose the service and the `ClipboardReader`.
- [x] The Wails-generated TypeScript frontend bindings remain method-compatible; no frontend renames.
- [x] The existing clipboard intake tests pass through the test adapter; the 1 Hz `Snapshot()` poll is unaffected.
- [x] `go test ./...` (and where applicable the frontend build) stays green; no user-visible behavior change.

## Comments

- Implemented by embedding `DownloadService` in `App`, removing service forwarders, and composing a Wails-backed `ClipboardReader`.
- Added `CannedClipboardReader` coverage for clipboard reads, review/confirmation, and explicit cancellation.
- Verification: `go test ./...`, `go vet ./...`, and `npm run build` pass.
