# aria2 Boundary Research

Source: [aria2c 1.37.0 manual](https://aria2.github.io/manual/en/html/aria2c.html), consulted 2026-07-31.

## Relevant constraints

- aria2 exposes JSON-RPC and XML-RPC over HTTP, plus JSON-RPC over WebSocket at `/jsonrpc`. WebSocket supports server notifications; HTTP does not.
- RPC listens on loopback by default. `--rpc-listen-all=false` must remain explicit, and `--rpc-secret` is the preferred authorization mechanism over deprecated basic-auth options.
- `aria2.shutdown` performs a graceful shutdown, while `aria2.forceShutdown` skips time-consuming cleanup. `aria2.saveSession` writes the configured session file.
- `--save-session` persists error and unfinished downloads, and `--save-session-interval` can persist them periodically. The session file can be supplied again with `--input-file` on restart.
- `.aria2` control files support resuming partial downloads. Session and control files therefore need to be treated as application data and protected from concurrent instances.
- RPC uses stable per-download GIDs, and queue insertion can specify a position. `tellStatus`, `tellActive`, `tellWaiting`, and `tellStopped` provide the state needed by the app adapter.

## Design implications

The app should launch one local aria2 process, choose a loopback RPC port, generate a per-process secret, and supervise the child process. The Go backend owns the process handle, RPC client, startup readiness check, graceful shutdown sequence, crash detection, and restart/reconciliation. The UI calls Go bindings only.

The backend owns product policy and user-visible metadata. aria2 owns transfer execution, GIDs, partial-file control state, and transport-level retry/progress. The adapter translates product operations into a narrow allowlisted set of JSON-RPC calls and validates responses before exposing them to the UI.

For the MVP, use JSON-RPC over HTTP with polling for status. Keep WebSocket notifications as a later optimization because HTTP RPC is sufficient for correctness and avoids adding a second event transport to the first implementation.

Run aria2 with RPC bound to loopback, a secret token, HTTPS-only download certificate verification, and a dedicated per-user data directory. Do not expose the RPC port or pass credentials through frontend code. The app must serialize session restore/startup and refuse a second managed instance for the same profile.

On normal exit, request `saveSession`, then `shutdown`, and wait for the child with a bounded timeout before force-terminating it. Configure periodic session saves so a process crash loses at most the configured interval of queue changes. On restart, load the session file, reconcile GIDs/statuses with the app's metadata, and surface unrecoverable entries rather than silently re-enqueueing them.
