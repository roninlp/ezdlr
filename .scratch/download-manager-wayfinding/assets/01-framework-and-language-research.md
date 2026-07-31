# Framework and Language Research

## Recommendation

Select **Wails v2 with Go for the application/backend and TypeScript for the UI**. Use a small frontend, preferably Svelte or vanilla TypeScript, and keep aria2 behind a Go-owned process/RPC adapter.

This is a recommendation for the stated MVP, not a claim that Wails is universally the lightest runtime. Ticket 06 must establish the project's actual cold-start and idle budgets.

## Why Wails

- Wails v2.13 documents Windows, Linux, and macOS support, native menus/dialogs, TypeScript model generation, JavaScript-to-Go bindings, and CLI packaging.
- Wails uses the platform webview rather than embedding a browser. That avoids Electron's bundled Chromium cost while preserving a mature HTML/CSS UI path.
- Wails documents no CGO or external DLL requirement on Windows, which reduces Windows packaging and toolchain risk.
- Go is a good fit for supervising a local aria2 process, JSON-RPC/WebSocket communication, filesystem persistence, and concurrent event handling without introducing Rust-specific complexity into the application boundary.
- Wails has a stable v2 API and established templates, which is a lower maintenance risk for this small MVP than the pre-1.0 Rust-native candidates.

## Candidate comparison

### Wails / Go

Best fit. Web UI maturity is high, the native shell is small, cross-platform support is explicit, and Go's process/concurrency model fits the aria2 adapter. Linux still depends on the host webview stack, so Arch packaging must validate WebKitGTK requirements.

### GPUI / Rust

Reject for the MVP. GPUI provides GPU-accelerated native UI and supports Linux (Wayland/X11) and Windows, but its own README says it is pre-1.0, actively developed for Zed, and subject to breaking changes. Documentation and widget breadth are also less mature than a web UI. It is attractive for a later performance-driven rewrite, not for the first implementation-ready specification.

### Dioxus / Rust

Reject for the MVP. Dioxus offers a productive Rust/RSX webview desktop path, cross-platform bundling, and a promising ecosystem. However, its native WGPU renderer is documented as experimental, while the webview path retains host-webview dependency concerns. It adds Rust/toolchain complexity without a decisive advantage for this download-manager UI.

### Tauri / TypeScript frontend + Rust backend

Reject as the primary choice, retain as the strongest alternative. Tauri has excellent small-binary architecture, explicit sidecar support, granular shell permissions, broad packaging support, and a larger ecosystem than the other Rust candidates. It is technically viable and may win if benchmark results show Wails misses the budget. For this MVP, requiring Rust for the backend increases implementation and maintenance cost without improving the core aria2 integration enough to justify switching.

### Electron / TypeScript

Reject. Electron offers the most mature UI and desktop ecosystem, but it bundles its own browser/runtime and conflicts directly with the low-idle-resource and small-runtime goals.

## Decision boundaries for later tickets

- Use Wails only as the desktop shell; do not put download policy in frontend code.
- Treat the host webview as a platform prerequisite to be documented and tested, not something the app silently assumes.
- Keep the aria2 adapter behind an internal interface so a future Tauri or native-shell migration does not alter queue semantics.
- Do not use framework marketing claims as performance requirements. Measure the shipped build on representative Arch and Windows environments in ticket 06.

## Sources

- [Wails introduction](https://wails.io/docs/introduction): platform support, webview architecture, bindings, templates, and packaging.
- [GPUI README](https://github.com/zed-industries/zed/tree/main/crates/gpui): pre-1.0 status, platform backends, and native UI architecture.
- [Dioxus README](https://github.com/DioxusLabs/dioxus): desktop rendering options, experimental native renderer, platforms, and bundling.
- [Tauri overview](https://v2.tauri.app/start/): system webview, small binary positioning, security, and cross-platform architecture.
- [Tauri architecture](https://v2.tauri.app/concept/architecture/): Rust/webview process boundary and supported tooling.
- [Tauri sidecars](https://v2.tauri.app/develop/sidecar/): external binary bundling, target triples, and permission-scoped process execution.
