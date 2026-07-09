# swedish-car-mcp

MCP servers for Swedish used EV / car buying workflows in Cursor and other MCP hosts.

## Packages

| Package | Description | Install |
|---------|-------------|---------|
| [bilmarknad-mcp](./bilmarknad-mcp/) | Search Blocket, Wayke, Tradera, Riddermark, Carla, and KVD for used cars | `uvx --from git+https://github.com/flyhard/swedish-car-mcp bilmarknad-mcp` |
| [aviloo-mcp](./aviloo-mcp/) | Parse AVILOO battery certificate PDFs in a project repo | `uvx --from git+https://github.com/flyhard/swedish-car-mcp aviloo-mcp` |

Pin a release with `@v0.1.0` on the git URL.

## Cursor (project `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "bilmarknad": {
      "command": "uvx",
      "args": [
        "--from",
        "git+https://github.com/flyhard/swedish-car-mcp@v0.1.0",
        "bilmarknad-mcp"
      ]
    },
    "aviloo": {
      "command": "uvx",
      "args": [
        "--from",
        "git+https://github.com/flyhard/swedish-car-mcp@v0.1.0",
        "aviloo-mcp"
      ],
      "env": {
        "AVILOO_MCP_REPO_ROOT": "${workspaceFolder}"
      }
    }
  }
}
```

## Development

```bash
cd bilmarknad-mcp && uv sync && uv run pytest -q
cd ../aviloo-mcp && uv sync && uv run pytest -q
```

## License

MIT — see [LICENSE](./LICENSE). Third-party marketplace APIs (Blocket, Wayke, Tradera, Riddermark, Carla, KVD) have their own terms; this project is unofficial and not affiliated with those services.
