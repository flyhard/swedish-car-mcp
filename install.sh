#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install swedish-car-mcp launchers that auto-update from GitHub Releases.

Usage:
  curl -fsSL https://raw.githubusercontent.com/flyhard/swedish-car-mcp/main/install.sh | bash
  install.sh [options]

Options:
  --prefix DIR    Install root (default: ~/.local or $SWEDISH_CAR_MCP_PREFIX)
  --force         Re-copy launchers even if present
  --no-download   Install wrappers only; skip initial binary download
  -h, --help      Show this help

Launchers are installed to $PREFIX/bin/{bilmarknad-mcp,aviloo-mcp}.
Shared logic lives in $PREFIX/share/swedish-car-mcp/.
Cached release binaries live in $PREFIX/share/swedish-car-mcp/cache/.

Environment:
  SWEDISH_CAR_MCP_PREFIX            Install root (same as --prefix)
  SWEDISH_CAR_MCP_VERSION           Pin version (e.g. v1.0.0); disables auto-update
  SWEDISH_CAR_MCP_UPDATE_INTERVAL   Seconds between update checks (default: 86400)
  SWEDISH_CAR_MCP_INSTALLER_REF     Git ref for installer files (default: main)
  SWEDISH_CAR_MCP_REPO              GitHub repo (default: flyhard/swedish-car-mcp)
  GITHUB_TOKEN                      Optional; raises GitHub API rate limits

After install, point Cursor mcp.json at:
  $PREFIX/bin/bilmarknad-mcp
  $PREFIX/bin/aviloo-mcp
EOF
}

PREFIX="${SWEDISH_CAR_MCP_PREFIX:-${HOME}/.local}"
FORCE=0
NO_DOWNLOAD=0
INSTALLER_REF="${SWEDISH_CAR_MCP_INSTALLER_REF:-main}"
REPO="${SWEDISH_CAR_MCP_REPO:-flyhard/swedish-car-mcp}"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/${INSTALLER_REF}/scripts"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="$2"; shift 2 ;;
    --force) FORCE=1; shift ;;
    --no-download) NO_DOWNLOAD=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 1 ;;
  esac
done

SCRIPT_DIR=""
_src="${BASH_SOURCE[0]:-}"
if [[ -n "$_src" && -f "$_src" ]]; then
  _dir="$(cd "$(dirname "$_src")" && pwd)"
  if [[ -f "$_dir/scripts/lib/common.sh" ]]; then
    SCRIPT_DIR="$(cd "$_dir/scripts" && pwd)"
  elif [[ -f "$_dir/lib/common.sh" ]]; then
    SCRIPT_DIR="$_dir"
  fi
fi
unset _src _dir

BIN_DIR="$PREFIX/bin"
SHARE_DIR="$PREFIX/share/swedish-car-mcp"
LIB_DIR="$SHARE_DIR/lib"
mkdir -p "$BIN_DIR" "$LIB_DIR"

install_local() {
  local src="$1" dest="$2"
  if [[ -f "$dest" && $FORCE -eq 0 ]]; then echo "exists: $dest"; return 0; fi
  cp "$src" "$dest"
  chmod +x "$dest" 2>/dev/null || true
  echo "installed: $dest"
}

install_remote() {
  local rel="$1" dest="$2"
  if [[ -f "$dest" && $FORCE -eq 0 ]]; then echo "exists: $dest"; return 0; fi
  mkdir -p "$(dirname "$dest")"
  curl -fsSL "${RAW_BASE}/${rel}" -o "$dest"
  chmod +x "$dest" 2>/dev/null || true
  echo "installed: $dest"
}

install_payload() {
  local rel="$1" dest="$2"
  if [[ -n "$SCRIPT_DIR" && -f "${SCRIPT_DIR}/${rel}" ]]; then
    install_local "${SCRIPT_DIR}/${rel}" "$dest"
  else
    install_remote "$rel" "$dest"
  fi
}

install_payload "lib/common.sh" "$LIB_DIR/common.sh"
install_payload "bilmarknad-mcp" "$BIN_DIR/bilmarknad-mcp"
install_payload "aviloo-mcp" "$BIN_DIR/aviloo-mcp"

export SWEDISH_CAR_MCP_SHARE_DIR="$SHARE_DIR"

if [[ $NO_DOWNLOAD -eq 0 ]]; then
  # shellcheck source=lib/common.sh
  source "$LIB_DIR/common.sh"
  echo "Fetching latest release..." >&2
  scm_ensure_release
  echo "Cached version: $(scm_installed_version)" >&2
fi

cat <<EOF

Done. Add to Cursor .cursor/mcp.json:

{
  "mcpServers": {
    "bilmarknad": {
      "command": "${BIN_DIR}/bilmarknad-mcp"
    },
    "aviloo": {
      "command": "${BIN_DIR}/aviloo-mcp",
      "env": {
        "AVILOO_MCP_REPO_ROOT": "\${workspaceFolder}"
      }
    }
  }
}

Launchers check GitHub Releases at most once per ${SWEDISH_CAR_MCP_UPDATE_INTERVAL:-86400}s.
Pin a version with SWEDISH_CAR_MCP_VERSION=v1.0.0 to disable auto-update.
EOF
