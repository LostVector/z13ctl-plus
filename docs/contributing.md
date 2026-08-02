# Contributing

Contributions are welcome. Please open an issue before starting work on a
significant change so the approach can be discussed first.

---

## Repository structure

This repo contains two Go modules:

| Module | Path | Purpose |
|--------|------|---------|
| `github.com/aic0d3r/z13ctl-plus` | `.` | Main CLI and daemon binary |
| `github.com/dahui/z13ctl/api` | `./api` | Public client library for external tools |

The `api/` module is stdlib-only so that GUI tools, Decky plugins, and other
integrations can import it without pulling in the CLI's dependencies.
The root module uses the fork identity. Only the separate API module retains the
canonical upstream import path for API and wire compatibility.

---

## Development setup

```sh
git clone https://github.com/aic0d3r/z13ctl-plus
cd z13ctl-plus
go mod download
```

To work on both modules together in your IDE or when making changes to `api/`,
create a `go.work` file (it is gitignored):

```sh
go work init . ./api
```

---

## Before submitting a pull request

```sh
make test      # run all tests
make lint      # run golangci-lint
make mod-tidy  # tidy go.mod for both modules
```

Tests do not require hardware. The `internal/aura` and `internal/cli`
packages are fully unit-testable. Code that interacts with `/dev/hidraw*`
is intentionally isolated in `internal/hid`.

Pull requests must pass both `make test` and `make lint` without errors and
should include tests for any new behavior.

---

## Testing notes

- `internal/aura` — fully unit-testable via mock writers; covers every packet type
- `internal/cli` — fully unit-testable; covers color parsing, dryrun output
- `internal/hid` — tests cover sysfs parsing; writes are tested via pipe-backed
  mock devices
- `internal/daemon` — state persistence is tested; server dispatch and button
  watcher require hardware or an evdev mock
- `cmd/` — no unit tests; integration tested manually against hardware

---

## Release workflow (maintainers only)

The API source retains the canonical `github.com/dahui/z13ctl/api` module path.
External Plus clients must use a replacement that selects the matching fork API
source because upstream API source targets the upstream runtime socket. Tagging
the root fork module does not publish the canonical API module path.

Create an annotated distribution tag to publish z13ctl-plus:

```sh
git tag -a vX.Y.Z -F release-notes.md
git push origin vX.Y.Z
```

GoReleaser handles binary builds and GitHub Release creation automatically
when the main module tag is pushed.

AUR publishing is a separate manual workflow dispatch and requires the
repository `AUR_KEY` secret.
