# facile

One installer for the whole Facile Studio suite. Pick your tools, keep them
current, and sign in once.

```sh
curl -fsSL https://get.facile.studio | bash
```

Installs to `~/.local/bin` and puts it on your `PATH` — zsh, bash and fish,
whichever the machine has, guarded so re-running changes nothing. Pass
`--bin-dir <dir>` to install elsewhere, `--no-path` to leave your shell config
alone, `--source` to build from source. `get.facile.studio` proxies this repo's
`install.sh` rather than holding a copy, so it is never out of date; the raw
GitHub URL keeps working as a fallback.

The terminal that ran the installer still has the old `PATH` — open a new one,
or `export PATH="$HOME/.local/bin:$PATH"` once.

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
facile login [tool...]     Sign in to every installed tool that has an account
facile logout [tool...]    Forget a stored credential, keep the server URL
```

`update` compares the version each installed binary reports against the latest
release tag and leaves the ones that already match alone; `--force` reinstalls
them anyway. `--all` takes the whole catalog. `--source` skips published
binaries and builds from source. `--no-skill` skips registering the tool's skill
with the AI coding agents on your machine. `facile list --json` prints one JSON
document.

## One login

```sh
facile login
```

With no arguments it signs in to every installed tool that has an account, so
you type one command rather than one per tool. `--pick` chooses from a list,
`--all` includes tools you have not installed.

The browser flows run first on purpose: they federate to the same identity
provider, so the first one asks for a password and the rest complete without
you touching anything. For the two tools that mint a key in a dashboard instead
of federating, facile opens that page rather than telling you to go find it.

`facile login <tool>` runs that tool's own sign-in flow and writes the result
**into the exact place that tool's CLI already reads from** — its keychain entry,
its YAML file, its JSON config. Nothing in the nine CLIs had to change for this
to work.

Which flow runs is data, not code: the catalog records each tool's discovery
endpoint, its loopback SSO parameters, its password endpoint or its device flow,
and facile does what the entry says. A tool with no account says so and exits 0,
because "capsule needs no login" is information, not a failure.

Files that hold more than a credential are read-modify-write. `nuage`'s config
also carries `sync_dir` and `ignore_patterns`, and an installer that reset a
user's sync directory to log them in would not be a convenience. Credentials are
created at `0600` rather than written and then chmodded, and a keychain that
cannot be reached falls back to a `0600` file with a warning instead of refusing
to store anything — a headless Linux box has no secret service, and that is not
a reason to have no login.

`facile logout` clears the credential and deliberately leaves the server URL, so
signing back in does not mean retyping where your instance lives.

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
- **The bootstrap fixes PATH; `facile` itself only warns.** `install.sh` is a
  first run, and a bin directory nothing can reach is not an install — so it
  writes a guarded prepend to the shells the machine has (zsh, bash, fish) and
  names each file it touched. `facile install` is the repeat operation, into a
  directory you already chose, so it says what is wrong and prints the line
  rather than editing your config behind you. Both report when a different
  binary of the same name comes first, which is the usual cause of "I updated
  it and nothing changed".
- **No state file.** What is installed is read off disk by running each binary,
  so facile's idea of your machine cannot drift from your machine.
- **The latest tag is resolved without the GitHub API**, by following the
  `/releases/latest` redirect. No rate limit, no token.

## License

MIT
