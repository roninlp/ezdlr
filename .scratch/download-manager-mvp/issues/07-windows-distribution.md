# 07 — Windows Distribution

**What to build:** Windows users can install and run the complete MVP through a self-contained NSIS installer with the application, pinned aria2 runtime, and normal desktop integration.

**Blocked by:** 06 — Arch Linux Distribution

**Status:** ready-for-agent

- [ ] The NSIS installer includes all required application and bundled aria2 runtime components and requires no Go, aria2, or other development dependency.
- [ ] The installed application invokes the private bundled engine deterministically and protects session, control, and credential-bearing data.
- [ ] The installation provides launcher integration, icons, system-tray entry, and uninstall support.
- [ ] A clean Windows installation can launch the application and exercise download, queue, persistence, and recovery behavior.
- [ ] Packaging verification confirms that file associations and in-app updating remain outside the MVP.
