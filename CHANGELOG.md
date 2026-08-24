# Changelog

All notable changes to this project are documented here. The format is
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). While on
`0.x`, a breaking change bumps the minor.

Entries before v0.6.0 were reconstructed from git history on 2026-08-24, so they
record what shipped rather than what was written down at the time.

## [Unreleased]

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

[Unreleased]: https://github.com/FacileStudio/facile/compare/v0.8.0...HEAD
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
