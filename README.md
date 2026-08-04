# ezdlr

ezdlr is a small native download manager for direct HTTP and HTTPS links. It
uses aria2 locally and keeps queue state between launches.

## Development

Requirements: Go, Node.js, npm, Wails, and aria2c. Linux development also
requires GTK 3 and WebKitGTK 4.1 development packages.

```sh
cd frontend && npm ci && cd ..
wails dev
```

Run the tests with:

```sh
go test ./...
```

## Releases

Push a version tag to build and publish Linux amd64 and Windows amd64 archives:

```sh
git tag v0.1.0-alpha.1
git push origin v0.1.0-alpha.1
```

Release archives include the ezdlr binary and its bundled aria2c executable.
