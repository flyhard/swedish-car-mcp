# swedish-car-mcp

MCP servers for Swedish used EV / car buying workflows in Cursor and other MCP hosts.

Both servers are implemented in **Go** and distributed as static binaries via [GitHub Releases](https://github.com/flyhard/swedish-car-mcp/releases).

## Install (recommended — auto-updating launchers)

One-time setup. `mcp.json` paths never change; launchers refresh from GitHub Releases in the background.

```bash
curl -fsSL https://raw.githubusercontent.com/flyhard/swedish-car-mcp/main/install.sh | bash
```

Or from a clone:

```bash
./install.sh
# or: ./scripts/install.sh
# or: make install-launchers
```

This installs:

| Path | Role |
|------|------|
| `~/.local/bin/bilmarknad-mcp` | Launcher (checks for updates, runs cached binary) |
| `~/.local/bin/aviloo-mcp` | Launcher |
| `~/.local/share/swedish-car-mcp/cache/` | Downloaded release binaries |

Launchers check for a newer release at most once per 24 hours. Pin a version to disable updates:

```bash
export SWEDISH_CAR_MCP_VERSION=v1.0.0
```

> **Note:** Auto-download requires at least one [GitHub Release](https://github.com/flyhard/swedish-car-mcp/releases). Before the first release, use `make build` and point `mcp.json` at `./bin/…` instead.

## Manual install from GitHub Releases

If you prefer a fixed binary without auto-update:

1. Open [Releases](https://github.com/flyhard/swedish-car-mcp/releases) and download the archive for your OS/arch (`darwin`/`linux`, `arm64`/`amd64`).
2. Extract `aviloo-mcp` and/or `bilmarknad-mcp` to a directory on your `PATH` (for example `~/bin`).

Or build locally:

```bash
make build
# binaries in ./bin/
```

## Cursor (project `.cursor/mcp.json`)

After `scripts/install.sh`, use the launcher paths (stable across releases):

```json
{
  "mcpServers": {
    "bilmarknad": {
      "command": "/Users/YOU/.local/bin/bilmarknad-mcp"
    },
    "aviloo": {
      "command": "/Users/YOU/.local/bin/aviloo-mcp",
      "env": {
        "AVILOO_MCP_REPO_ROOT": "${workspaceFolder}"
      }
    }
  }
}
```

Replace `/Users/YOU` with your home directory, or run `echo $HOME/.local/bin/bilmarknad-mcp` after install.

## Environment variables

### bilmarknad-mcp

| Variable | Purpose |
|----------|---------|
| `WAYKE_API_KEY` | Optional Wayke REST bearer token |
| `BLOCKET_PROXY_URL` | Optional Blocket search proxy base URL |
| `TRADERA_APP_ID` | Tradera developer app ID |
| `TRADERA_APP_KEY` | Tradera developer app key |
| `TRADERA_CAR_CATEGORY_ID` | Tradera car category (default `10`) |

### aviloo-mcp

| Variable | Purpose |
|----------|---------|
| `AVILOO_MCP_REPO_ROOT` | Repo root for PDF lookup (set to workspace in Cursor) |

### Launcher (install.sh)

| Variable | Purpose |
|----------|---------|
| `SWEDISH_CAR_MCP_VERSION` | Pin version (e.g. `v1.0.0`); disables auto-update |
| `SWEDISH_CAR_MCP_UPDATE_INTERVAL` | Seconds between update checks (default `86400`) |
| `SWEDISH_CAR_MCP_PREFIX` | Install root (default `~/.local`) |
| `SWEDISH_CAR_MCP_REPO` | GitHub repo (default `flyhard/swedish-car-mcp`) |
| `GITHUB_TOKEN` | Optional; raises GitHub API rate limits |

## Development

```bash
make test             # go test -race -cover ./...
make build            # build both binaries to ./bin/
make install-launchers  # install auto-updating wrappers to ~/.local/bin
make tidy             # go mod tidy
```

Release binaries are built with [GoReleaser](https://goreleaser.com/) on git tags matching `v*`.

## MCP tools

**bilmarknad-mcp:** `search_cars`, `get_listing`, `list_sources`

**aviloo-mcp:** `extract_aviloo_pdf`, `lookup_aviloo_cert`, `list_aviloo_pdfs`

## License

MIT — see [LICENSE](./LICENSE). Third-party marketplace APIs (Blocket, Wayke, Tradera, Riddermark, Carla, KVD) have their own terms; this project is unofficial and not affiliated with those services.
