# Changelog

All notable changes to this project are documented here. The format is
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). While on
`0.x`, a breaking change bumps the minor.

Entries before v0.6.0 were reconstructed from git history on 2026-08-24, so they
record what shipped rather than what was written down at the time.

## [Unreleased]

### Added

- **`facile login` speaks the OIDC device flow (RFC 8628).** A sign-in on a
  machine whose browser lives elsewhere no longer hangs. facile prints a short
  code and a URL, the user opens the URL on any device, and the terminal
  completes on its own — nothing is redirected to a loopback port on a machine
  that is not the one running the browser.

  The loopback flow assumed otherwise, and every one of the six federated tools
  had the defect: the provider redirected the *browser's* machine to
  `127.0.0.1:<port>`, where nothing listened, and the login code expired unused
  while facile waited for a callback that could never arrive.

  One authorization covers the whole run. The tools share an identity provider,
  so the first asks for a code and the rest complete silently.

  It is a new auth kind, `oidc-device`, distinct from the existing per-tool
  `device` kind, which is a tool's own start/poll endpoints and a different
  protocol. Endpoints come from the provider's discovery document, never from
  paths assembled by facile: only the root `/.well-known/openid-configuration`
  is real on this provider, and a per-application path answers 200 with the
  single-page app's HTML.

### Changed

- sablier, nuage, casier, journal, courrier and mycelium move from `kind: sso`
  to `kind: oidc-device` in the catalog. Each keeps its `sso` block: the
  loopback flow is the same-machine fast path, and it is also what runs until
  the provider lists the device grant in `grant_types_supported`. facile asks
  rather than assumes, so the change is inert until the server side is ready
  and needs no second release to switch on.

## [0.9.1] — 2026-08-24

### Fixed

- `list` compared versions for difference rather than order, so a binary newer
  than the cached tag rendered as `facile 0.9.0 → 0.8.0` — an arrow pointing
  backwards at a downgrade, counted in the footer as an available update. The
  cache is a day old by design and the running binary can be ahead of it, which
  made this routine for facile's own row and latent for every other. Comparison
  is now an ordering on the parsed semver, which also fixes `0.10.0` sorting
  below `0.9.0` as a string.
- `facile update` on a Homebrew install resolved nothing before printing
  `brew upgrade --cask facile`, so it said that on every run whether or not
  there was anything to upgrade to, and left the version cache untouched. It
  now checks first and reports being up to date when it is.

## [0.9.0] — 2026-08-24

### Added

- facile is the first row of `facile list`, and `facile update facile` replaces
  the running binary in place — its own binary, at its own path, atomically,
  which is what CLI-STANDARD §3.1 permits. A bare `facile update` takes facile
  along with everything installed.

  It refuses under Homebrew. brew records the version it staged, so overwriting
  the file leaves brew's manifest claiming the old one and the next
  `brew upgrade` re-stages from that record and reverts the update. It prints
  `brew upgrade --cask facile` instead.

  It also leaves a source build alone unless `--force` says otherwise, rather
  than replacing a locally built binary with a release.
- `facile doctor` reports other facile binaries on `PATH`. The existing shadow
  check compares against the catalog bin dir and cannot see this: a self update
  writes to the running binary's own directory, so a second copy earlier on
  `PATH` keeps answering with the old version and the update looks like it did
  nothing.

### Fixed

- `list` no longer claims a source build is out of date. A binary reporting a
  commit SHA was rendered as `edf2b6f → 0.25.0` and counted in the footer, which
  is not a comparison anyone can make — the SHA may be ahead of the tag. Worse,
  the footer sent the reader to `facile update`, which would replace their build
  with the release. `update` keeps treating the same unknown as a reinstall on
  purpose: there the cost of being wrong is a download, not a false statement.
- `facile install facile` and `facile uninstall facile` explain themselves
  instead of reporting facile as an unknown tool, which read as a bug once the
  listing started showing it.
- The update-count footer no longer prints with a leading indent.

## [0.8.0] — 2026-08-24

### Added

- facile reports its own version. `list` prints `facile 0.7.0 → 0.8.0` under
  the table when a newer release is published, and `doctor` reports the version
  either way, resolving the tag live rather than from the cache.

  The upgrade command follows the install method: a Homebrew cask resolves into
  the brew prefix and is upgraded with `brew`, while `facile update` writes to
  `~/.local/bin`. Naming the wrong one leaves two copies and a `PATH` race,
  which is the failure facile warns about everywhere else.

  facile is deliberately **not** a catalog entry. The catalog is the input to
  `install`, `update`, `uninstall` and `--all`, so a row for facile would offer
  to uninstall the running binary. An outdated facile is also not a `doctor`
  problem and cannot fail the command: it installs tools perfectly well, and a
  health check that went red on every release would be red more often than
  useful.

### Fixed

- The README tool table listed 8 of the 12 catalogued tools.

## [0.7.0] — 2026-08-24

### Added

- `facile list` marks an installed tool that has a newer release as
  `0.4.2 → 0.5.0` and prints a count under the table. The published versions
  come from a new `~/.cache/facile/latest.json`, refreshed at most once a day,
  so the listing stays instant and still works offline; `--check` resolves them
  now instead. `--json` gains `latest` and `outdated` per tool, and `--quiet` never
  touches the network at all.

  `update` still resolves the latest tag live. It decides whether to download,
  and that decision must not read a day-old cache.
- `douane` joins the tool catalog.

### Fixed

- `--no-color` now applies to an error raised while parsing the arguments.
  Colors were set in `PersistentPreRun`, which cobra never reaches when a flag
  is rejected, so the one output the flag exists to control was the one output
  it could not reach.

### Removed

- `spore` leaves the catalog and the tool table; the repository was deleted
  upstream. The installer test now uses `filet` as its fixture.

## [0.6.2] — 2026-08-24

### Changed

- Homebrew installs ship as a cask. The formula path goreleaser used is
  deprecated, and `homebrew_casks.binary` is deprecated with it — the cask
  declares a `binaries` list instead.
- CI actions move off the node 20 runtime.

## [0.6.1] — 2026-08-24

### Fixed

- The Homebrew formula now publishes. `release.yml` resolved
  `{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}` without ever mapping the secret into
  the goreleaser step, so v0.6.0 published its GitHub release in full and then
  died on a template error. Pins the action to `~> v2` while there.

## [0.6.0] — 2026-08-24

### Added

- `facile update` compares the version each installed binary reports against the
  latest release tag and leaves the ones that already match alone. A no-op run
  drops from about 32 seconds to about 1.
- `facile update --force` reinstalls regardless of the comparison.
- `nacelle`, the terminal coding agent, joins the catalog.

### Changed

- Anything the version comparison cannot establish reinstalls: a tool that is
  not installed, one with no release asset, an unreadable tag, or a `--version`
  line that is not `<bin> <semver>`. A broken check degrades to the previous
  behaviour rather than silently refusing to update anyone.

## [0.5.0] — 2026-08-23

### Added

- `courrier` and `journal` join the catalog; `capsule`, `antenne` and `ardoise`
  gain skill registration.
- Skills are written to the mycelium/pi target as well as Claude Code and Codex.
- `install.sh` puts the bin dir on PATH for zsh, bash and fish instead of
  warning about it. The bootstrap is the one documented exception to the
  "warn, never fix" rule, because it runs before there is a facile to warn.

### Fixed

- The release path no longer fails silently. A checksum mismatch aborts instead
  of falling back to a source build, and the fallback warning prints the
  underlying cause rather than one fixed sentence for three unrelated failures.
- `facile update` refreshes the catalog before choosing targets, so a
  newly catalogued tool no longer needs a second run.
- `casier`'s catalog entry described a stale loopback-token SSO contract.
- The `journal` credential points at `config.yml`, where the CLI reads it.

## [0.4.0] — 2026-08-10

### Added

- `mycelium` and `nuage` sign in through the browser.

### Fixed

- A catalog cache written by the previous binary no longer survives an upgrade,
  which used to hide the very change the user had just installed.

## [0.3.0] — 2026-08-10

### Changed

- `facile login` takes every installed tool by default and prefers browser
  flows.

## [0.2.3] — 2026-08-10

### Fixed

- `sablier`'s SSO flow requires its nonce.

## [0.2.2] — 2026-08-10

### Fixed

- `facile doctor` no longer reports a symlinked bin dir as its own impostor.

## [0.2.1] — 2026-08-10

### Added

- `ardoise` ships prebuilt binaries.

## [0.2.0] — 2026-08-10

### Added

- `facile login` and `facile logout`: one login for the suite, written where
  each tool reads it.

## [0.1.0] — 2026-08-10

### Added

- First release. One installer for the whole suite, with a bootstrap script,
  tests and CI.

[Unreleased]: https://github.com/FacileStudio/facile/compare/v0.9.1...HEAD
[0.9.1]: https://github.com/FacileStudio/facile/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/FacileStudio/facile/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/FacileStudio/facile/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/FacileStudio/facile/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/FacileStudio/facile/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/FacileStudio/facile/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/FacileStudio/facile/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/FacileStudio/facile/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/FacileStudio/facile/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/FacileStudio/facile/compare/v0.2.3...v0.3.0
[0.2.3]: https://github.com/FacileStudio/facile/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/FacileStudio/facile/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/FacileStudio/facile/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/FacileStudio/facile/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/FacileStudio/facile/releases/tag/v0.1.0
