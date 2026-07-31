# Minimal Cross-Platform Download Manager Wayfinding Map

Status: Open
Labels: wayfinder:map

## Destination

An implementation-ready technical specification selecting the language/framework and defining the architecture for a minimal, fast download manager, with Arch Linux as the first target, Windows next, and HTTP/HTTPS downloads powered initially by aria2.

## Notes

Domain: desktop download management and native cross-platform application architecture.

Consult framework, aria2, Linux desktop integration, packaging, and performance documentation while resolving tickets. Prefer minimal dependencies, low idle resource use, and a reliable queue-manager experience over feature breadth. This map plans the work; it does not implement the application.

## Decisions so far

- [Which language and desktop framework should be selected for a minimal, fast Linux-first download manager that must have a credible Windows path?](issues/01-framework-and-language.md) — Select Wails v2 with Go plus a TypeScript UI; reject GPUI, Dioxus, and Electron for MVP risk or resource reasons, while retaining Tauri as the measured fallback.
- [What application/aria2 boundary should the specification adopt?](issues/02-aria2-boundary.md) — Own one local aria2 child per profile behind a Go JSON-RPC adapter; supervise and restore it through loopback RPC, secret auth, session saves, and control files.
- [What exact MVP behavior constitutes a reliable queue manager?](issues/03-queue-and-concurrency.md) — Use FIFO with manual queued-item movement, a three-download global cap, four connections per download, bounded three-attempt retries, preserved partials on pause/cancel, and restore-and-continue restart behavior.
- [How should clipboard batch intake work in the MVP?](issues/04-clipboard-batch-intake.md) — Inspect only on user action; extract valid HTTP/HTTPS URLs from mixed text, skip known duplicates, review before enqueueing, use the configured destination, and show categorized results.
- [Given the selected framework and aria2 boundary, what packaging, installation, update, and runtime-dependency strategy should the specification use for Arch Linux first and Windows next?](issues/05-linux-packaging-and-windows-path.md) — Bundle a pinned aria2 per platform; use an AUR-compatible native PKGBUILD for Arch and NSIS for Windows, with launcher/icon/uninstall/tray integration and no MVP file associations or in-app updater.
- [How should “minimal and fast” be made testable?](issues/06-performance-budget-and-validation.md) — Validate Wails with broad startup, idle-resource, and responsiveness criteria across launch, clipboard batch, and three-download restart workflows using lightweight OS/app measurements; compare rejected frameworks qualitatively.

## Not yet specified

<!-- No remaining in-scope fog. -->

## Out of scope

- Implementing the download manager during this wayfinding effort; the destination is a specification for a later implementation effort.
- Browser integration, browser interception, BitTorrent, Metalink, FTP, authentication, scheduling, and macOS support are deferred beyond this MVP specification.
- Browser extensions, browser interception, BitTorrent, Metalink, FTP, and macOS first-release support unless a later decision explicitly redraws the destination.
