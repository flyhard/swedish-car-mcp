# bilmarknad-mcp

MCP server for searching Swedish used car marketplaces from AI assistants (Cursor, Claude Desktop, etc.).

## Sources

- **Blocket** — direct mobility search API (optional `blocket-api.se` proxy fallback)
- **Wayke** — REST with optional `WAYKE_API_KEY`, or public GraphQL
- **Tradera** — car auctions and buy-now listings via REST API v3 (cached; 100 API calls/day)
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
| `TRADERA_APP_ID` | No | Tradera developer app ID (shared dev default if unset) |
| `TRADERA_APP_KEY` | No | Tradera developer app key (register at [api.tradera.com](https://api.tradera.com)) |
| `TRADERA_CAR_CATEGORY_ID` | No | Tradera category for car search (default `10` = Bilar) |

## Development

```bash
uv sync
uv run pytest -q
uv run bilmarknad-mcp
```

## Example prompts

- "Search Kia Niro EV under 300 000 kr on Blocket, Wayke, and Tradera"
- "Get listing details from this Tradera URL: …"
- "Which sources are configured?"


## Battery state of health (SoH)

Listings may include optional SoH fields parsed from ad text (no structured API field on Blocket/Wayke):

| Field | Type | Description |
|-------|------|-------------|
| `soh_percent` | float | Battery state of health (0–100), e.g. from `99% SoH` or `91.4% batterihälsa` |
| `battery_tested` | bool | `true` when text mentions batteritestad, Aviloo, hälsotillstånd, etc. |
| `soh_source` | string | Where SoH was found: `blocket_search`, `blocket_detail`, `wayke_search`, `wayke_detail`, `tradera_search`, `tradera_detail` |
| `soh_raw_match` | string | Substring that matched the percent pattern |

- **Search results** — Blocket parses `model_specification`, heading, extras, labels; Wayke parses title and `shortDescription`.
- **Single listing (`get_listing`)** — Blocket fetches `blocket-api.se/v1/ad/car` for subtitle/equipment; Wayke uses a detail GraphQL query for description and `data.properties` / `data.options`; Tradera fetches item details via REST API v3.

## Tradera rate limits

Tradera allows roughly **100 API calls per 24 hours** per app credentials. This MCP caches search and item responses for 30 minutes. Register your own app at [api.tradera.com](https://api.tradera.com) to avoid sharing the default development credentials.

## Disclaimer

Unofficial integration with third-party marketplaces. Respect their terms of service; APIs may change without notice.
