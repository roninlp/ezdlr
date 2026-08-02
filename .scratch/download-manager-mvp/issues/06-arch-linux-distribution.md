# 06 — Arch Linux Distribution

**What to build:** Arch Linux users can install and launch the complete MVP through an AUR-compatible native package with all runtime and desktop integration components included.

**Blocked by:** 02 — Managed aria2 Single-Download Flow; 04 — Persistence and Restart Recovery

**Status:** completed

- [x] An AUR-compatible native package installs the application and its pinned, application-compatible aria2 binary without requiring a system aria2 installation.
- [x] The installed application invokes its private bundled engine deterministically and stores runtime data in the expected per-user locations.
- [x] The package provides a desktop launcher, icons, and uninstall support through pacman.
- [x] A clean Arch installation can launch the application and exercise download, queue, persistence, and recovery behavior.
- [x] Package verification documents the required host webview/runtime dependencies and confirms no unsupported MVP features are implied.

## Comments

### Implementation

Added `packaging/arch/PKGBUILD` with a private pinned aria2 1.37.0 build,
desktop integration, and package verification notes. The application now only
accepts only the executable `aria2c` bundled with the application package,
uses `$HOME/Downloads` and the per-user config directory, and disables WebKit GPU compositing through the
Wails Linux option so Wayland startup does not depend on
`WEBKIT_DISABLE_DMABUF_RENDERER=1`.
