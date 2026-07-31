# 06 — Arch Linux Distribution

**What to build:** Arch Linux users can install and launch the complete MVP through an AUR-compatible native package with all runtime and desktop integration components included.

**Blocked by:** 02 — Managed aria2 Single-Download Flow; 04 — Persistence and Restart Recovery

**Status:** ready-for-agent

- [ ] An AUR-compatible native package installs the application and its pinned, application-compatible aria2 binary without requiring a system aria2 installation.
- [ ] The installed application invokes its private bundled engine deterministically and stores runtime data in the expected per-user locations.
- [ ] The package provides a desktop launcher, icons, system-tray entry, and uninstall support.
- [ ] A clean Arch installation can launch the application and exercise download, queue, persistence, and recovery behavior.
- [ ] Package verification documents the required host webview/runtime dependencies and confirms no unsupported MVP features are implied.
