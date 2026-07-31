Status: Closed
Labels: wayfinder:research
Parent: ../spec.md
Assignee: opencode

## Question

Which language and desktop framework should be selected for a minimal, fast Linux-first download manager that must have a credible Windows path? Compare Wails/Go, GPUI/Rust, Dioxus/Rust, and TypeScript desktop alternatives against startup time, idle memory, UI maturity, Linux/Arch support, Windows portability, aria2 integration, packaging, and maintenance risk. Produce a recommendation and explicit rejected alternatives.

## Comments

### Resolution

Select **Wails v2 with Go for the application/backend and TypeScript for the UI**. Keep aria2 behind a Go-owned process/RPC adapter and keep download policy out of frontend code. This is the lowest-risk fit for the MVP's Linux-first, Windows-next, low-resource goals: Wails uses the host webview, provides mature HTML/CSS UI options and Go bindings, supports the target desktop platforms, and avoids CGO or external DLL requirements on Windows.

Reject GPUI because it is explicitly pre-1.0 with breaking-change risk and a less mature widget/documentation ecosystem. Reject Dioxus because its native renderer is experimental and its webview path does not provide a decisive advantage here. Reject Electron because bundling Chromium conflicts with the resource goal. Retain Tauri as the strongest alternative if measurement shows Wails misses the budget; its Rust backend, sidecar support, permissions, and packaging are technically strong, but Rust adds unnecessary implementation and maintenance cost for this MVP.

Research asset: [Framework and Language Research](../assets/01-framework-and-language-research.md)
