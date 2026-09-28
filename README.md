# ezdlr

ezdlr is a small native download manager for direct HTTP and HTTPS links. It
uses aria2 locally and keeps queue state between launches.

## Development

Requirements: Go 1.25+, Node.js, npm, the Wails v3 CLI, and aria2c. Linux
development also requires GTK 3 and WebKitGTK 4.1 development packages; the
Linux build selects that stack with the `gtk3` build tag.

Install the Wails CLI. It is only a build tool, so it does not need the GUI
dependencies and can be built without cgo:

```sh
CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Run the app in development mode:

```sh
wails3 dev
```

Development builds run `aria2c` from `PATH`, so install aria2 before starting
the app. Release builds require the `aria2c` binary bundled next to the
executable instead.

Regenerate the TypeScript bindings after changing the methods or models bound
to the frontend:

```sh
wails3 task common:generate:bindings
```

Type-check the frontend with the TypeScript compiler:

```sh
cd frontend && npm run typecheck
```

Build the application into `bin/ezdlr`:

```sh
wails3 build
```

Run the tests with:

```sh
go test -tags gtk3 ./...
```

The tests compile the Wails application, so build the frontend first (`wails3
build` does this automatically) to populate `frontend/dist`. The `gtk3` tag is
required on Linux because Wails v3 defaults to the GTK 4 / WebKitGTK 6.0 stack;
`wails3 build` and `wails3 dev` add the tag for you.

## Releases

Push a version tag to build and publish Linux amd64, Windows amd64, and macOS
arm64 archives:

```sh
git tag v0.1.0-alpha.1
git push origin v0.1.0-alpha.1
```

Release archives include the ezdlr binary and its bundled aria2c executable. The
macOS archives contain an application bundle that can be opened from Finder.

The AppImage also requires GTK 3 and WebKitGTK 4.1 from the host system.
