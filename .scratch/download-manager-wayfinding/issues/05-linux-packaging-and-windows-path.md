Status: Closed
Labels: wayfinder:grilling
Parent: ../spec.md
Blocked by: 02-aria2-boundary.md
Assignee: opencode

## Question

Given the selected framework and aria2 boundary, what packaging, installation, update, and runtime-dependency strategy should the specification use for Arch Linux first and Windows next? Decide how aria2 is obtained and versioned, whether system packages are supported, and what minimum desktop integration is required without making distribution the first-release bottleneck.

## Comments

### Resolution

Bundle and pin the application-compatible `aria2` binary for each supported platform; do not require a system-installed `aria2` for the MVP. Ship Arch Linux first as a native, AUR-compatible `PKGBUILD`, with updates delivered through the package manager. Ship Windows with a Wails-default NSIS installer containing the application and bundled `aria2`; defer an in-app auto-updater.

Both packages must provide an application launcher, icons, uninstall support, and a system-tray entry. File associations are deferred beyond the MVP. The package/runtime layout must keep the bundled `aria2` private to the application and let the Go supervisor invoke the pinned binary deterministically.
