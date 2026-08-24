#!/usr/bin/env bash
#
# Bootstrap installer for facile, the Facile Studio suite installer.
#
# Every statement sits inside a function and main() is the last line, so a
# download truncated mid-flight executes nothing at all. This is the single
# most important property of a `curl | bash` script and it is not negotiable.
#
# Once facile is on your PATH, it installs everything else:
#   facile install

set -euo pipefail

REPO="FacileStudio/facile"
BIN="facile"

# --- output -----------------------------------------------------------------

setup_colors() {
  if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
    C_INFO=$'\033[36m' C_OK=$'\033[32m' C_WARN=$'\033[33m' C_ERR=$'\033[31m'
    C_DIM=$'\033[2m' C_OFF=$'\033[0m'
  else
    C_INFO="" C_OK="" C_WARN="" C_ERR="" C_DIM="" C_OFF=""
  fi
}

info() { printf '%s▸%s %s\n' "$C_INFO" "$C_OFF" "$*"; }
ok()   { printf '%s✓%s %s\n' "$C_OK" "$C_OFF" "$*"; }
warn() { printf '%s!%s %s\n' "$C_WARN" "$C_OFF" "$*" >&2; }
hint() { printf '  %s%s%s\n' "$C_DIM" "$*" "$C_OFF"; }
die()  { printf '%s✗%s %s\n' "$C_ERR" "$C_OFF" "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 not found — $2"; }

usage() {
  cat <<EOF
Install facile, the Facile Studio suite installer.

Usage:
  install.sh [options]

Options:
  --bin-dir <dir>   Directory to install into (default: ~/.local/bin)
  --version <tag>   Release tag to install (default: latest)
  --source          Build from source, ignore published releases
  --no-path         Do not touch shell config files to put the bin dir on PATH
  -h, --help        Show this help

Environment:
  FACILE_BIN_DIR    Same as --bin-dir
  FACILE_NO_PATH    Same as --no-path
  NO_COLOR          Disable colored output
EOF
}

# Colors are set before argument parsing: parse_args rejects unknown options
# through die(), which prints with them, and under `set -u` an unset C_ERR
# kills the script before its own error message renders.
setup_colors

# --- steps ------------------------------------------------------------------

parse_args() {
  BIN_DIR="${FACILE_BIN_DIR:-$HOME/.local/bin}"
  VERSION=""
  FROM_SOURCE=0
  # Writing to shell config by default is what rustup, bun and uv do, and the
  # alternative — printing a line and hoping it gets read — is the failure this
  # exists to fix. CI is the one place it is wrong: a runner does not want a
  # ~/.bashrc conjured for it, and it sets its own PATH anyway.
  SETUP_PATH=1
  [ -z "${CI:-}" ] || SETUP_PATH=0
  [ -z "${FACILE_NO_PATH:-}" ] || SETUP_PATH=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --bin-dir) BIN_DIR="${2:?--bin-dir needs a value}"; shift 2 ;;
      --bin-dir=*) BIN_DIR="${1#*=}"; shift ;;
      --version) VERSION="${2:?--version needs a value}"; shift 2 ;;
      --version=*) VERSION="${1#*=}"; shift ;;
      --source) FROM_SOURCE=1; shift ;;
      --no-path) SETUP_PATH=0; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown option: $1 — this script installs facile itself; run 'facile install <tool>' for suite tools" ;;
    esac
  done
  BIN_DIR="${BIN_DIR%/}"
  PATH_CHANGED=0
  PATH_ALREADY=0
}

detect_platform() {
  case "$(uname -s)" in
    Linux) OS=linux ;;
    Darwin) OS=darwin ;;
    *) die "unsupported operating system: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

make_workdir() {
  WORK="$(mktemp -d)"
  trap 'rm -rf "$WORK"' EXIT
  mkdir -p "$WORK/out"
}

prepare_bin_dir() {
  mkdir -p "$BIN_DIR" 2>/dev/null || die "cannot create $BIN_DIR"
  [ -w "$BIN_DIR" ] || die "$BIN_DIR is not writable"
}

# latest_tag follows the /releases/latest redirect and reads the tag out of the
# effective URL. No GitHub API, so no rate limit and no token.
latest_tag() {
  curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/$REPO/releases/latest" 2>/dev/null |
    sed -n 's#.*/releases/tag/##p'
}

install_from_release() {
  [ "$FROM_SOURCE" -eq 0 ] || return 1
  command -v curl >/dev/null 2>&1 || return 1
  command -v tar >/dev/null 2>&1 || return 1

  local tag ver archive base
  tag="$VERSION"
  [ -n "$tag" ] || tag="$(latest_tag)"
  [ -n "$tag" ] || return 1

  ver="${tag#v}"
  archive="facile_${ver}_${OS}_${ARCH}.tar.gz"
  base="https://github.com/$REPO/releases/download/$tag"

  info "Downloading facile $ver for $OS/$ARCH"
  curl -fsSL -o "$WORK/$archive" "$base/$archive" 2>/dev/null || return 1
  curl -fsSL -o "$WORK/checksums.txt" "$base/checksums.txt" 2>/dev/null || return 1
  verify_checksum "$WORK" "$archive" || die "checksum mismatch for $archive"

  tar -xzf "$WORK/$archive" -C "$WORK/out" || return 1
  [ -f "$WORK/out/$BIN" ] || return 1
  atomic_install "$WORK/out/$BIN" "$BIN_DIR/$BIN"
}

# verify_checksum aborts on a mismatch and never falls back. A wrong hash means
# something is wrong with the artifact, not with the network.
verify_checksum() {
  local dir="$1" file="$2" sum
  if command -v sha256sum >/dev/null 2>&1; then
    sum="$(cd "$dir" && sha256sum "$file")"
  elif command -v shasum >/dev/null 2>&1; then
    sum="$(cd "$dir" && shasum -a 256 "$file")"
  else
    warn "no sha256 tool available, skipping checksum verification"
    return 0
  fi
  grep -qF "${sum%% *}  $file" "$dir/checksums.txt"
}

install_from_source() {
  need git "install git first"
  need go "install Go from https://go.dev/dl"

  info "Fetching source"
  git clone --depth 1 --quiet "https://github.com/$REPO.git" "$WORK/src" ||
    die "cannot clone $REPO"

  info "Building from source"
  local ver
  ver="$(git -C "$WORK/src" describe --tags --always 2>/dev/null || echo dev)"
  (cd "$WORK/src" && go build -trimpath -ldflags "-s -w -X main.version=${ver#v}" \
    -o "$WORK/out/$BIN" .)
  atomic_install "$WORK/out/$BIN" "$BIN_DIR/$BIN"
}

# atomic_install stages beside the destination and renames, so replacing facile
# while it is running cannot corrupt the running image.
atomic_install() {
  local src="$1" dest="$2" staged
  staged="$(dirname "$dest")/.$(basename "$dest").new.$$"
  install -m 755 "$src" "$staged" || die "cannot write $staged"
  mv -f "$staged" "$dest" || { rm -f "$staged"; die "cannot write $dest"; }
}

# --- PATH ---------------------------------------------------------------------
#
# A bin dir nothing can reach is not an install. The suite standard is
# ~/.local/bin, which macOS does not put on PATH by default, so persisting it is
# part of the job rather than a warning in the output that nobody reads.

PATH_MARKER='# facile installer'

# path_literal keeps the entry portable: written as $HOME/... it survives a home
# directory that moves and a dotfile repo shared across machines.
path_literal() {
  case "$BIN_DIR" in
    "$HOME"/*) printf '$HOME/%s' "${BIN_DIR#"$HOME"/}" ;;
    *) printf '%s' "$BIN_DIR" ;;
  esac
}

# path_snippet guards the prepend at *runtime* rather than at write time. Config
# files get sourced twice in more setups than you would expect — a .bash_profile
# that sources .bashrc, a login shell inside tmux — and a runtime guard makes
# that harmless instead of leaving duplicates in PATH.
path_snippet() {
  local kind="$1" dir="$2"
  case "$kind" in
    fish)
      printf '\n%s\nif not contains %s $PATH\n    set -gx PATH %s $PATH\nend\n' \
        "$PATH_MARKER" "$dir" "$dir"
      ;;
    *)
      printf '\n%s\ncase ":$PATH:" in\n  *":%s:"*) ;;\n  *) export PATH="%s:$PATH" ;;\nesac\n' \
        "$PATH_MARKER" "$dir" "$dir"
      ;;
  esac
}

# uses_shell is true when the machine has the shell at all: the binary exists,
# it is the login shell, or it has already left a config file behind.
uses_shell() {
  local name="$1" f
  shift
  command -v "$name" >/dev/null 2>&1 && return 0
  [ "$(basename "${SHELL:-}")" = "$name" ] && return 0
  for f in "$@"; do
    [ -e "$f" ] && return 0
  done
  return 1
}

# path_targets prints "kind:file" per shell present. zsh, bash and fish are what
# people actually log in with; anything else falls back to the printed export.
path_targets() {
  local zdot fishrc f wrote
  zdot="${ZDOTDIR:-$HOME}"
  fishrc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish"

  if uses_shell zsh "$zdot/.zshrc"; then
    printf 'posix:%s\n' "$zdot/.zshrc"
  fi

  # macOS starts bash as a login shell, which reads .bash_profile and never
  # .bashrc; Linux does the opposite. Writing whichever already exists covers
  # both, and .bashrc is the right one to create when neither does.
  if uses_shell bash "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; then
    wrote=0
    for f in "$HOME/.bashrc" "$HOME/.bash_profile"; do
      if [ -f "$f" ]; then
        printf 'posix:%s\n' "$f"
        wrote=1
      fi
    done
    [ "$wrote" -eq 1 ] || printf 'posix:%s\n' "$HOME/.bashrc"
  fi

  if uses_shell fish "$fishrc"; then
    printf 'fish:%s\n' "$fishrc"
  fi

  return 0
}

# configure_path never fails the install. The binary is already on disk and
# usable by absolute path; an unwritable dotfile is worth a warning, not an
# aborted installation.
configure_path() {
  [ "$SETUP_PATH" -eq 1 ] || return 0
  case ":$PATH:" in
    *":$BIN_DIR:"*) return 0 ;;
  esac

  local literal targets target kind file dir
  literal="$(path_literal)"
  targets="$(path_targets)"
  [ -n "$targets" ] || return 0

  while IFS= read -r target; do
    [ -n "$target" ] || continue
    kind="${target%%:*}"
    file="${target#*:}"

    if [ -f "$file" ] &&
       grep -qF -e "$PATH_MARKER" -e "$literal" -e "$BIN_DIR" "$file" 2>/dev/null; then
      PATH_ALREADY=$((PATH_ALREADY + 1))
      continue
    fi

    dir="$(dirname "$file")"
    mkdir -p "$dir" 2>/dev/null || { warn "cannot create ${dir/#$HOME/\~}"; continue; }

    if path_snippet "$kind" "$literal" >> "$file" 2>/dev/null; then
      ok "PATH set in ${file/#$HOME/\~}"
      PATH_CHANGED=$((PATH_CHANGED + 1))
    else
      warn "cannot write ${file/#$HOME/\~}"
    fi
  done <<< "$targets"
}

# --- report -----------------------------------------------------------------

report() {
  local version shadow
  version="$("$BIN_DIR/$BIN" --version 2>/dev/null | head -n1)" ||
    die "$BIN installed to $BIN_DIR/$BIN but does not run"
  ok "${version:-$BIN} installed to ${BIN_DIR/#$HOME/\~}/$BIN"

  configure_path

  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *)
      # A script cannot change the PATH of the shell that invoked it, so even a
      # successful write needs one manual step for the terminal already open.
      # Saying "not on your PATH" when the config already has it is worse than
      # useless: it invites a second copy of the line, added by hand.
      if [ "$PATH_CHANGED" -gt 0 ]; then
        info "Open a new terminal, or run this once in the current one:"
      elif [ "$PATH_ALREADY" -gt 0 ]; then
        info "Already in your shell config — open a new terminal, or run:"
      else
        warn "${BIN_DIR/#$HOME/\~} is not on your PATH"
      fi
      hint "export PATH=\"$BIN_DIR:\$PATH\""
      ;;
  esac

  shadow="$(command -v "$BIN" 2>/dev/null || true)"
  if [ -n "$shadow" ] && [ "$shadow" != "$BIN_DIR/$BIN" ]; then
    warn "another $BIN comes first on your PATH: $shadow"
  fi

  info "Run \`facile install\` to pick your tools"
}

# --- main -------------------------------------------------------------------

main() {
  parse_args "$@"
  info "Installing facile"
  detect_platform
  make_workdir
  prepare_bin_dir
  install_from_release || install_from_source
  report
}

main "$@"
