# facile

One installer for the whole Facile Studio suite. Pick your tools, keep them
current, and sign in once.

```sh
curl -fsSL https://raw.githubusercontent.com/FacileStudio/facile/main/install.sh | bash
```

Installs to `~/.local/bin`. Pass `--bin-dir <dir>` to change that, `--source` to
build from source.

Then:

```sh
facile install
```

## Commands

```
facile install [tool...]   Install tools, or open a picker with no arguments
facile list                Show the catalog and what is installed
facile update [tool...]    Update installed tools to their latest release
facile uninstall <tool>    Remove a tool's binary
facile doctor              Check PATH, shadowed binaries and interrupted installs
```

`--all` takes the whole catalog. `--source` skips published binaries and builds
from source. `--no-skill` skips registering the tool's skill with the AI coding
agents on your machine. `facile list --json` prints one JSON document.

## The catalog

| Tool | What it is |
|---|---|
| `opus` | Project management |
| `sablier` | Time tracking |
| `nuage` | Cloud storage sync |
| `casier` | Secrets and environment variables |
| `capsule` | End-to-end encrypted paste |
| `antenne` | Alert log and delivery targets |
| `mycelium` | Shared agent memory |
| `spore` | Monorepo package manager |
| `ardoise` | Invoice and contract PDFs |

The catalog lives in [`internal/manifest/tools.yml`](internal/manifest/tools.yml).
It is embedded at build time and refreshed from `main` at runtime, so a tenth
tool never means reinstalling facile.

## Why this exists

Every suite CLI used to carry its own copy of a 273-line `install.sh`. The rule
was that everything below a nine-line config block stayed byte-identical in all
nine repos. It stopped being true within four days: one repo gained a fix that
stages the new binary and renames it into place — so that updating a tool while
it is running cannot corrupt the running image — and the other eight never got
it. Copy-paste is not a distribution mechanism.

Each repo's `install.sh` is now a shim that bootstraps facile and delegates, so
there is one implementation and nothing left to drift.

## Design notes

- **One install directory for the suite: `~/.local/bin`.** Never `sudo`, never
  outside `$HOME` by default.
- **Prebuilt binary first, source build second.** Checksums are mandatory on the
  download path, and a mismatch aborts rather than falling back — a wrong hash
  is a problem with the artifact, not with the network.
- **Verify by running.** facile reports the version the installed binary itself
  printed. An installer that claims success without executing the artifact is
  lying.
- **Warn, never fix, on PATH.** If `~/.local/bin` is missing from `PATH`, or a
  different binary of the same name comes first, facile says so and prints the
  line to add. It does not edit your shell configuration.
- **No state file.** What is installed is read off disk by running each binary,
  so facile's idea of your machine cannot drift from your machine.
- **The latest tag is resolved without the GitHub API**, by following the
  `/releases/latest` redirect. No rate limit, no token.

## License

MIT
