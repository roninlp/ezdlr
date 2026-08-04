# 01 — Collapse six engine capability interfaces into one deep DownloadEngine interface

**What to build:** The download engine is represented by one deep interface on the existing engine seam instead of six one-method capability interfaces. The download service stops type-asserting the engine at each call site and calls the single interface; the engine adapter implements all of it. The test doubles stop embedding ladders of partial interface implementations and instead implement the single deep interface directly, with the base fake growing the remaining methods as trivial canned returns. A behavior change to the engine contract becomes a compile error at one interface rather than a new type-assertion discovered at every call site. From the user's perspective no behavior changes; the refactor is verifiable by the absence of type-assertions in the service, the absence of embed-ladder fakes across the test files, and a green test suite.

**Blocked by:** None — can start immediately.

**Status:** completed

- [x] One deep `DownloadEngine` interface exists on the existing engine seam, carrying add, status, pause, resume, cancel, recover, exited, GID, and shutdown.
- [x] All eleven type-assertion call sites in the download service (across `service.go` and `clipboard.go`) call the single interface directly; no type-assertion remains.
- [x] `Aria2Engine` satisfies the single deep `DownloadEngine` interface (it already implements every method today; no new production behavior).
- [x] The embed-ladder fakes (`queueFakeEngine`, `exitFakeEngine`, `recoveryFakeEngine`) collapse into direct implementations of the single interface; the base `FakeEngine` grows the remaining methods with canned returns (`Status` returns `EngineStatus{}`, lifecycle methods return nil, `Recover` returns an empty slice, `Exited` returns a nil channel, `GID` returns empty).
- [x] The six narrow capability interfaces are deleted.
- [x] `go test ./...` stays green with the consolidated fakes; no behavior change is observable through the service boundary.

## Comments

- Implemented in `fc3e858` by consolidating the engine contract and updating production and test adapters.
