# Changelog

All notable changes to this project are documented here. The format is
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). While on
`0.x`, a breaking change bumps the minor.

Entries before v0.6.0 were reconstructed from git history on 2026-08-24, so they
record what shipped rather than what was written down at the time.

## [Unreleased]

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

[Unreleased]: https://github.com/FacileStudio/facile/compare/v0.6.1...HEAD
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
