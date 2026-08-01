# 02 — Managed aria2 Single-Download Flow

**What to build:** A valid direct HTTP or HTTPS URL downloads through exactly one application-owned, bundled aria2 process, while the queue view shows trustworthy engine-backed status and progress.

**Blocked by:** 01 — Bootable App and Download-Service Seam

**Status:** completed

- [x] The application launches a compatible aria2 binary path and communicates with it through loopback JSON-RPC only.
- [x] Each managed process uses a private per-process RPC secret, HTTPS certificate verification, and a dedicated private runtime data area.
- [x] The service waits for RPC readiness, validates responses, maps engine identifiers, polls status, and reports progress and transfer counters.
- [x] The UI can add and observe a real direct HTTP or HTTPS download without communicating with aria2 directly.
- [x] Engine adapter contract tests cover RPC serialization, readiness, status mapping, and bounded process shutdown.

## Comments

### Implementation

Implemented in commits `0276f5e`, `6afea4c`, and `64a396e`.

- Added the managed aria2 JSON-RPC adapter and process lifecycle supervision.
- Added loopback-only RPC, per-process secrets, certificate verification, private runtime/session data, and profile locking.
- Added engine GID mapping, status reconciliation polling, progress counters, and frontend refresh polling.
- Added adapter contract tests and a real local aria2 readiness/shutdown test.
- Platform-specific bundled binary packaging and pinning remain tracked by issue 07.

Validation: `go test ./...`, `go vet ./...`, and `npm run build` from `frontend/` passed.
