# Arch Linux package

The `PKGBUILD` fetches the tagged Wails application, then installs a pinned private `aria2c` 1.37.0
binary, the desktop launcher, and the application icon. `aria2c` is installed
under `/usr/lib/ezdlr` and is started by its private package path; a
system `aria2` installation is not used.

The host must provide WebKitGTK 4.1 and its normal GTK runtime dependencies.
The application selects Wails' software WebView GPU policy, so Wayland users
do not need `WEBKIT_DISABLE_DMABUF_RENDERER=1` or another launcher override.

Build with:

```sh
makepkg -si
```

After installation, verify the package with `desktop-file-validate`, launch
`ezdlr` from the desktop menu, add an HTTP or HTTPS URL, pause/resume it, and
restart the app. The queue state should be restored from the per-user config
directory. BitTorrent, FTP, authentication, browser interception, and file
associations are intentionally not provided by this package.

Removal is handled by pacman (`pacman -R ezdlr`), including the desktop
launcher and icon. User queue state and downloaded files remain in the user's
home directory and are not removed by package uninstall.
