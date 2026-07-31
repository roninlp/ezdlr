Status: Closed
Labels: wayfinder:research
Parent: ../spec.md
Assignee: opencode

## Question

What application/aria2 boundary should the specification adopt? Decide whether the app launches and owns a local aria2 process or connects to an externally managed daemon, and define the required RPC model, process lifecycle, configuration ownership, error recovery, persistence, and security treatment for a local HTTP/HTTPS download manager.

## Comments

### Resolution

Adopt an **app-owned local aria2 process** rather than connecting to an externally managed daemon. The Go backend launches exactly one aria2 child for the user profile, selects a loopback RPC port, generates a per-process RPC secret, waits for RPC readiness, supervises the process, and exposes a narrow download-service API to the TypeScript UI. The UI never connects to aria2 directly.

Use JSON-RPC over HTTP at `/jsonrpc` for the MVP and poll status through the Go adapter. Keep WebSocket notifications as a later optimization, not a correctness dependency. The adapter owns RPC serialization, request timeouts, response validation, GID mapping, startup restore, and crash/restart reconciliation.

The Go application owns user-facing queue policy, metadata, configuration, and lifecycle. aria2 owns transfer execution, partial-file control files, transport retries, progress counters, and GIDs. Start aria2 with loopback-only RPC, `--rpc-secret`, HTTPS certificate verification, a dedicated per-user data directory, and the configured download directory. Prevent concurrent managed instances for one profile.

On normal exit, call `aria2.saveSession`, then `aria2.shutdown`, wait with a bounded timeout, and force-terminate only if needed. Enable periodic session saves so crashes lose at most the configured interval of queue changes. Restore with `--input-file` plus aria2 control files, then reconcile the restored GIDs and statuses against app metadata; never silently duplicate an entry. Treat session, control, and any credential-bearing files as private application data.

Research asset: [aria2 Boundary Research](../assets/02-aria2-boundary-research.md)
