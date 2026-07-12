# Shared helpers for swedish-car-mcp launchers.

SWEDISH_CAR_MCP_REPO="${SWEDISH_CAR_MCP_REPO:-flyhard/swedish-car-mcp}"
SWEDISH_CAR_MCP_UPDATE_INTERVAL="${SWEDISH_CAR_MCP_UPDATE_INTERVAL:-86400}"

scm_share_dir() {
  if [[ -n "${SWEDISH_CAR_MCP_SHARE_DIR:-}" ]]; then
    echo "$SWEDISH_CAR_MCP_SHARE_DIR"
    return
  fi
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
}

scm_cache_dir() {
  echo "${SWEDISH_CAR_MCP_CACHE_DIR:-$(scm_share_dir)/cache}"
}

scm_detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    MINGW* | MSYS* | CYGWIN* | Windows_NT) echo "windows" ;;
    *)
      echo "unsupported OS: $(uname -s)" >&2
      return 1
      ;;
  esac
}

scm_detect_arch() {
  case "$(uname -m)" in
    arm64 | aarch64) echo "arm64" ;;
    x86_64 | amd64) echo "amd64" ;;
    *)
      echo "unsupported arch: $(uname -m)" >&2
      return 1
      ;;
  esac
}

scm_platform() {
  echo "$(scm_detect_os)_$(scm_detect_arch)"
}

scm_binary_filename() {
  local name="$1"
  if [[ "$(scm_detect_os)" == "windows" ]]; then
    echo "${name}.exe"
  else
    echo "$name"
  fi
}

scm_archive_name() {
  local version="$1"
  if [[ "$(scm_detect_os)" == "windows" ]]; then
    echo "swedish-car-mcp_${version}_$(scm_platform).zip"
  else
    echo "swedish-car-mcp_${version}_$(scm_platform).tar.gz"
  fi
}

scm_github_api() {
  local path="$1"
  local tmp http_code
  tmp="$(mktemp)"
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    http_code="$(curl -sSL -w "%{http_code}" -o "$tmp" \
      -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/vnd.github+json" \
      "https://api.github.com${path}")"
  else
    http_code="$(curl -sSL -w "%{http_code}" -o "$tmp" \
      -H "Accept: application/vnd.github+json" "https://api.github.com${path}")"
  fi
  if [[ "$http_code" != "200" ]]; then
    if [[ "$http_code" == "404" ]]; then
      echo "no GitHub release found for ${SWEDISH_CAR_MCP_REPO} (HTTP 404)" >&2
      echo "Publish a release first, or re-run install with --no-download." >&2
    else
      echo "GitHub API error ${http_code} for ${path}" >&2
    fi
    rm -f "$tmp"
    return 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

scm_resolved_version() {
  if [[ -n "${SWEDISH_CAR_MCP_VERSION:-}" ]]; then
    echo "${SWEDISH_CAR_MCP_VERSION#v}"
    return 0
  fi
  local raw tag
  raw="$(scm_github_api "/repos/${SWEDISH_CAR_MCP_REPO}/releases/latest")" || return 1
  tag="$(printf '%s' "$raw" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v?([^"]+)".*/\1/')"
  if [[ -z "$tag" ]]; then
    echo "could not parse latest release tag" >&2
    return 1
  fi
  echo "$tag"
}

scm_version_file() {
  echo "$(scm_cache_dir)/current-version"
}

scm_last_check_file() {
  echo "$(scm_cache_dir)/.last-check"
}

scm_should_check_update() {
  if [[ -n "${SWEDISH_CAR_MCP_VERSION:-}" ]]; then
    return 1
  fi
  local last_check now interval last_check_file
  last_check_file="$(scm_last_check_file)"
  interval="$SWEDISH_CAR_MCP_UPDATE_INTERVAL"
  now="$(date +%s)"
  if [[ ! -f "$last_check_file" ]]; then
    return 0
  fi
  last_check="$(cat "$last_check_file" 2>/dev/null || echo 0)"
  [[ $((now - last_check)) -ge interval ]]
}

scm_mark_checked() {
  mkdir -p "$(scm_cache_dir)"
  date +%s >"$(scm_last_check_file)"
}

scm_installed_version() {
  local vf
  vf="$(scm_version_file)"
  if [[ -f "$vf" ]]; then
    cat "$vf"
  fi
}

scm_binary_path() {
  local name="$1"
  local version="${2:-$(scm_installed_version)}"
  [[ -n "$version" ]] || return 1
  echo "$(scm_cache_dir)/${version}/$(scm_binary_filename "$name")"
}

scm_binary_ready() {
  local path="$1"
  if [[ "$(scm_detect_os)" == "windows" ]]; then
    [[ -f "$path" ]]
  else
    [[ -x "$path" ]]
  fi
}

scm_verify_checksum() {
  local archive_path="$1"
  local version="$2"
  local archive_name checksums_url tmp expected actual
  archive_name="$(basename "$archive_path")"
  checksums_url="https://github.com/${SWEDISH_CAR_MCP_REPO}/releases/download/v${version}/checksums.txt"
  tmp="$(mktemp)"
  if ! curl -fsSL "$checksums_url" -o "$tmp" 2>/dev/null; then
    rm -f "$tmp"
    return 0
  fi
  expected="$(awk -v f="$archive_name" '$2 == f {print $1; exit}' "$tmp")"
  rm -f "$tmp"
  [[ -n "$expected" ]] || return 0
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive_path" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
  else
    return 0
  fi
  if [[ "$expected" != "$actual" ]]; then
    echo "checksum mismatch for $archive_name" >&2
    return 1
  fi
}

scm_download_version() {
  local version="$1"
  local archive dest_dir archive_path url
  archive="$(scm_archive_name "$version")"
  dest_dir="$(scm_cache_dir)/${version}"
  archive_path="$(scm_cache_dir)/${archive}"
  mkdir -p "$(scm_cache_dir)" "$dest_dir"

  url="https://github.com/${SWEDISH_CAR_MCP_REPO}/releases/download/v${version}/${archive}"
  echo "Downloading ${url}" >&2
  curl -fsSL "$url" -o "$archive_path"

  if ! scm_verify_checksum "$archive_path" "$version"; then
    rm -f "$archive_path"
    return 1
  fi

  if [[ "$(scm_detect_os)" == "windows" ]]; then
    unzip -o "$archive_path" -d "$dest_dir"
  else
    tar -xzf "$archive_path" -C "$dest_dir"
  fi
  rm -f "$archive_path"
  echo "$version" >"$(scm_version_file)"

  for bin in bilmarknad-mcp aviloo-mcp; do
    local bin_path="$dest_dir/$(scm_binary_filename "$bin")"
    if [[ -f "$bin_path" ]]; then
      chmod +x "$bin_path" 2>/dev/null || true
    fi
  done
}

scm_ensure_release() {
  local installed want
  installed="$(scm_installed_version)"

  if [[ -n "$installed" ]] && scm_binary_ready "$(scm_binary_path bilmarknad-mcp "$installed")"; then
    if ! scm_should_check_update; then
      return 0
    fi
    scm_mark_checked
    want="$(scm_resolved_version)" || return 1
    if [[ "$want" == "$installed" ]]; then
      return 0
    fi
    scm_download_version "$want"
    return 0
  fi

  want="$(scm_resolved_version)" || return 1
  scm_download_version "$want"
  scm_mark_checked
}

scm_ensure_binary() {
  local name="$1"
  scm_ensure_release
  local path
  path="$(scm_binary_path "$name")"
  if ! scm_binary_ready "$path"; then
    echo "binary not found: $path" >&2
    return 1
  fi
  echo "$path"
}
