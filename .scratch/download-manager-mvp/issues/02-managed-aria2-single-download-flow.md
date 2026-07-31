# 02 — Managed aria2 Single-Download Flow

**What to build:** A valid direct HTTP or HTTPS URL downloads through exactly one application-owned, bundled aria2 process, while the queue view shows trustworthy engine-backed status and progress.

**Blocked by:** 01 — Bootable App and Download-Service Seam

**Status:** ready-for-agent

- [ ] The application launches a pinned, compatible aria2 binary and communicates with it through loopback JSON-RPC only.
- [ ] Each managed process uses a private per-process RPC secret, HTTPS certificate verification, and a dedicated private runtime data area.
- [ ] The service waits for RPC readiness, validates responses, maps engine identifiers, polls reconciled status, and reports progress and transfer counters.
- [ ] The UI can add and observe a real direct HTTP or HTTPS download without communicating with aria2 directly.
- [ ] Engine adapter contract tests cover RPC serialization, readiness, status mapping, and bounded process shutdown.
