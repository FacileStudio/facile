# Plan: Universal Package Manager — External Sources for Facile

## Goal
Allow `facile` to install tools from arbitrary GitHub repositories by reading a per-repo `facile.toml` (canonical) or `[facile]` block in `mise.toml` (fallback), with optional user-defined sources in `~/.config/facile/sources.yml`. The catalog becomes the union of the embedded catalog + user sources + discovered repos.

## Why (evidence)
- Current `tools.yml` is hand-maintained; adding a tool requires a FacileStudio PR.
- Every suite repo already has `mise.toml` with build info (`[tasks.build]`, `[tasks.install]`); the data exists but isn't machine-readable for facile.
- Users want to manage their own CLIs through `facile` without forking the suite catalog.
- The installer (`buildFor`, `fromRelease`, `fromSource`) already handles arbitrary repos — only the manifest source is hardcoded.

## Approach
1. **Add `facile.toml` schema** — mirrors `manifest.Tool` fields, plus optional `versionCmd`/`versionPattern` for `update`/`list`. `mise.toml` with `[facile]` block is a fallback (checked if `facile.toml` absent).
2. **User sources files** — two locations, checked in priority order:
   - `~/facile.yml` (highest)
   - `~/.config/facile/sources.yml` (XDG fallback)
   Each has `single:` (explicit Tool entries) and `lists:` (URLs returning repo identifiers).
3. **Discovery pipeline** — at catalog load, merge layers in order: embedded → remote → user single (from first-found sources file) → user list-discovered (fetch `facile.toml` → `mise.toml`/`[facile]` at `main`/`master` per repo). List sources support both one-repo-per-line and PEP 503 simple index (HTML with `<a href="/owner/repo/">`).
4. **Shadowing rule** — later layers override earlier on `Tool.Name` (same semantics as `FACILE_CATALOG`).
5. **No new commands** — `install`, `update`, `list`, `doctor` work unchanged; they just see more tools.

## Steps (ordered)

1. `internal/manifest/types.go` (new) — move `Tool`, `Manifest`, `Auth` out of `manifest.go`; add `SourceConfig`, `SingleSource`, `ListSource`, `FacileToml` types. [filet]
2. `internal/manifest/facile_toml.go` (new) — parse `facile.toml` and `[facile]` from `mise.toml` into `FacileToml`; convert to `Tool` with defaults. [filet]
3. `internal/manifest/sources.go` (new) — load user sources: try `~/facile.yml`, then `~/.config/facile/sources.yml`; first one found wins. Parse `single:` and `lists:`. Fetch list sources (HTTP GET, support both one-repo-per-line and PEP 503 simple index). For each repo, fetch `facile.toml` at `main`/`master`; if absent, fetch `mise.toml` and parse `[facile]`; convert valid ones to `Tool`. Cache list responses 24h. [filet]
4. `internal/manifest/merge.go` (new) — `Merge(base *Manifest, layers ...*Manifest) *Manifest` implementing shadow-by-name; used by `Load`. [filet]
5. `internal/manifest/manifest.go` — refactor `Load` to: load embedded → fetch remote → load user sources → merge all layers → return. Keep `FACILE_CATALOG` override as highest-priority layer. [filet]
6. `internal/installer/release.go` — add `versionCmd`/`versionPattern` to `Tool`; update `upToDate`/`matchesVersion` to use them when present (fallback to current heuristic). [filet]
7. `cmd/root.go` — update `catalog()` to use new merged `Load`; ensure `FACILE_CATALOG` still wins. [filet]
8. `internal/store/store.go` — add `SourcesPaths()` returning `[~/facile.yml, ~/.config/facile/sources.yml]` in priority order; `SourcesPath()` returns the first that exists (or XDG for writes). Ensure dir exists. [filet]
9. `cmd/doctor.go` — add check for sources file validity (parse + reachable list URLs). Report:
   - which sources file is active (path)
   - parse errors (if any)
   - list URLs that are unreachable or return non-2xx
   - repos from lists that have no `facile.toml` and no `[facile]` in `mise.toml`
   - user tools that shadow catalog tools (name collision) — this is the useful signal [filet]
10. `install.sh` — document `facile.toml` + `sources.yml` in comments (no behavior change). [filet]
11. Example `facile.toml` and `sources.yml` committed to `docs/` for reference. [filet]

## Files to Modify / New
- **New**: `internal/manifest/types.go`, `internal/manifest/facile_toml.go`, `internal/manifest/sources.go`, `internal/manifest/merge.go`
- **Modified**: `internal/manifest/manifest.go`, `internal/manifest/auth.go` (if `Auth` moves to `types.go`), `internal/installer/release.go`, `cmd/root.go`, `internal/store/store.go`, `cmd/doctor.go`, `install.sh`
- **Docs**: `docs/facile.toml.example`, `docs/sources.yml.example`

## Exit criteria
- `go build ./...` succeeds
- `filet check .` passes (no new violations)
- `go test ./...` passes
- Manual: `facile install` shows tools from a test `sources.yml` with a local repo
- Manual: `facile update` correctly compares versions for a tool with `versionCmd`/`versionPattern`
- Manual: `facile doctor` reports source health

## Risks / unknown unknowns
- **List source auth** — private repos need a token; `sources.yml` could support `token:` per list, but that's a secret in config. Defer to env var or keychain in v2.
- **List source rate limits** — GitHub raw URLs are unauthenticated and rate-limited; a list with 50 repos will hit limits. Add caching (24h, like catalog) and parallel fetch with backoff.
- **Mise.toml parsing** — TOML with `[facile]` is easy; but repos without `mise.toml` or with malformed TOML should fail silently (skip, not error).
- **Version detection** — many CLIs print non-semver lines; `versionPattern` is a regex, but a bad regex makes `update` think it's current. Validate pattern at load time.
- **Shadowing surprises** — a user source named `sablier` overrides the suite tool. Warn in `doctor` when a user tool shadows a catalog tool.
- **Cross-repo `goVersionVar`** — suite tools use `main.version`; external tools may use `github.com/user/repo/cmd.version` or nothing. `facile.toml` makes it explicit.

## Skip (YAGNI)
- **MCP/skill registration for external tools** — requires the external tool to implement the suite's install subcommand protocol. Out of scope for v1.
- **Auth flow for external tools** — `login`/`logout` only work for tools with `Auth` block in `tools.yml`. External tools would need their own `Auth` in `facile.toml`, which couples to the suite's credstore formats. Defer.
- **Publish/registry hosting** — this plan is *consumption* only. A hosted registry (like a FacileStudio-maintained list) is a separate infra decision.
- **Non-GitHub hosts** — GitHub raw URLs cover 99% of cases. GitLab/Bitbucket can be added when someone asks.
- **Binary transparency / sigstore** — checksum verification exists for release assets; source builds are trust-on-first-use. Not in v1.
- **`facile.toml` validation schema** — JSON Schema or CUE could validate; `filet` on the repo itself is the existing gate. Skip.

## Checked against
- [filet] — all new code must pass `filet check`; existing limits unchanged.
- [module-path] — module stays `github.com/FacileStudio/facile`; no new packages outside `internal/manifest`, `internal/installer`, `cmd`, `internal/store`.
- [auth/porte] — no auth changes; external tools don't get `login` support in v1.
- [events] — no event emission added.
- [distribute] — no cross-repo deps.
- [migrations] — N/A (no DB).
- [muse] — N/A (no UI).

## Pause for review
**Decisions confirmed:**
1. `facile.toml` canonical + `mise.toml` fallback ✓
2. Sources path priority: `~/facile.yml` → `~/.config/facile/sources.yml` ✓
3. List source format: both one-repo-per-line and PEP 503 simple index ✓
4. Shadowing warning in `doctor` — useful signal (warn on name collision) ✓

Ready to execute.