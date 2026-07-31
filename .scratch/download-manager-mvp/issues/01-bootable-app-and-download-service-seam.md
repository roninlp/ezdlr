# 01 — Bootable App and Download-Service Seam

**What to build:** A native Wails application that opens directly to a usable queue view, with a Go-owned download-service boundary and a demonstrable single-link flow backed by a controllable fake engine.

**Blocked by:** None — can start immediately.

**Status:** completed

- [x] The application launches to a usable queue view without setup screens.
- [x] The service exposes intake, queue, lifecycle, status, configuration, and recovery concepts using the defined download states.
- [x] A user can submit a valid direct HTTP or HTTPS URL and see the resulting queue item through the UI.
- [x] Deterministic service tests cover the initial user-visible flow through the fake engine boundary.
- [x] Download policy remains in the Go service rather than being duplicated in the UI.

## Comments

### Implementation

Implemented the initial Wails v2 application shell, Go download-service boundary, in-memory fake engine, direct URL validation, queue snapshot, default configuration, and TypeScript queue UI.

Service tests pass with `go test -race service.go service_test.go`. Full module tests and the frontend build now pass after the development dependencies were made available. The Wails build uses the `webkit2_41` tag for Arch Linux's WebKitGTK 4.1 package and produces the Linux executable successfully.
