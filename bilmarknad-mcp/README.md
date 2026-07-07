# bilmarknad-mcp

MCP server for searching Swedish used car marketplaces from AI assistants (Cursor, Claude Desktop, etc.).

## Sources

- **Blocket** — direct mobility search API (optional `blocket-api.se` proxy fallback)
- **Wayke** — REST with optional `WAYKE_API_KEY`, or public GraphQL
- **KVD** — probe only (returns empty until a stable public API exists)

## Tools

| Tool | Description |
|------|-------------|
| `search_cars` | Unified search with make/model/price/year/mileage filters |
| `get_listing` | Fetch one listing by `source`+`id` or public URL |
| `list_sources` | Show configured sources and env vars |

## Install (uvx)

```bash
uvx --from 'git+https://github.com/flyhard/swedish-car-mcp@v0.1.0#subdirectory=bilmarknad-mcp' bilmarknad-mcp
```

## Cursor

```json
{
  "mcpServers": {
    "bilmarknad": {
      "command": "uvx",
      "args": [
        "--from",
        "git+https://github.com/flyhard/swedish-car-mcp@v0.1.0#subdirectory=bilmarknad-mcp",
        "bilmarknad-mcp"
      ]
    }
  }
}
```

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `WAYKE_API_KEY` | No | Wayke REST bearer token |
| `BLOCKET_PROXY_URL` | No | Blocket search proxy base (default `https://blocket-api.se`) |

## Development

```bash
uv sync
uv run pytest -q
uv run bilmarknad-mcp
```

## Example prompts

- "Search Kia Niro EV under 300 000 kr on Blocket and Wayke"
- "Get listing details from this Blocket URL: …"
- "Which sources are configured?"

## Disclaimer

Unofficial integration with third-party marketplaces. Respect their terms of service; APIs may change without notice.
