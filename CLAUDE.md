# facile

The installer and credential front door for the Facile Studio suite. Go + cobra,
one binary, no server.

## What it is

`facile` installs, updates and removes every suite CLI, and signs the user in to
the ones that have accounts. It is the single implementation of "install a Facile
tool" — each product repo's `install.sh` is a shim that bootstraps facile and
delegates to it.

The normative rules live in `~/Projects/Facile/Wiki/CLI-STANDARD.md`. When this
repo and that document disagree, the document wins.

## Layout

```
main.go                     version var, set by ldflags
cmd/                        one file per command, cobra
internal/manifest/          the catalog: tools.yml, its types, and the auth contract
internal/installer/         download, verify, extract, source build, skill registration
internal/store/             where things live on disk (bin dir, config, cache)
internal/ui/                the five output levels of CLI-STANDARD §4
install.sh                  bootstrap for facile itself
```

## The catalog is the product

`internal/manifest/tools.yml` is canonical and is embedded with `go:embed`. It is
also refreshed at runtime from its raw URL on `main`, so adding a tool does not
require anybody to reinstall facile. There is exactly one copy of this file — do
not add a second at the repo root "for convenience".

Its `auth:` block is **transcribed from each CLI's own credential read path**, not
designed. A wrong service string or a missing `/api` suffix means facile writes a
token the tool will never find, and the user sees a successful login followed by
401s. When a CLI changes where it reads its credential, this file changes in the
same breath.

## Rules that are load-bearing

- **Verify by running.** Never report a version that was not printed by the
  installed binary itself.
- **A checksum mismatch aborts.** It never falls back to a source build.
- **Atomic install.** Stage beside the destination, then rename. Writing over a
  running binary corrupts it, and that bug is the reason this repo exists.
- **No state file.** What is installed is discovered by running the binaries.
- **Warn, never fix, on PATH.** The binary never edits the user's shell
  configuration; `reportPath` prints and stops. `install.sh` is the one
  exception, and prepends the bin dir for zsh, bash and fish, because the
  bootstrap runs before there is a facile to do the warning.
- **Never sudo, never write outside `$HOME`** unless `--bin-dir` says so.

## Style

Biome-free; `gofmt` and `go vet` are the gate. No inline comments — a comment
earns its place by explaining *why*, above a declaration. Output goes through
`internal/ui`, never `fmt.Println`. Errors are lowercase, name what failed, and
end with what to do about it after an em dash.

## Commands

```sh
go build ./...
go test ./...
go vet ./...
bash -n install.sh
```
