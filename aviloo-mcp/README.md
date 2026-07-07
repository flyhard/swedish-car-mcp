# aviloo-mcp

Repo-local MCP server (no network) for parsing **AVILOO** independent battery health certificate PDFs stored in your project.

## Tools

| Tool | Description |
|------|-------------|
| `extract_aviloo_pdf` | Parse one PDF path (must stay under repo root) |
| `lookup_aviloo_cert` | Find PDF by certificate UUID or 32-char download id |
| `list_aviloo_pdfs` | List AVILOO-like PDFs under the repo |

## Prerequisites

- Python 3.11+
- `pdftotext` (poppler) **or** `pypdf` (installed as dependency)

## Install (uvx)

```bash
uvx --from 'git+https://github.com/flyhard/swedish-car-mcp@v0.1.0#subdirectory=aviloo-mcp' aviloo-mcp
```

## Cursor

```json
{
  "mcpServers": {
    "aviloo": {
      "command": "uvx",
      "args": [
        "--from",
        "git+https://github.com/flyhard/swedish-car-mcp@v0.1.0#subdirectory=aviloo-mcp",
        "aviloo-mcp"
      ],
      "env": {
        "AVILOO_MCP_REPO_ROOT": "${workspaceFolder}"
      }
    }
  }
}
```

`AVILOO_MCP_REPO_ROOT` defaults to the current working directory when not set.

## Cert UUID vs download id

AVILOO PDFs are often saved as `{download_id}.pdf` (32 hex chars). The certificate UUID is inside the PDF, e.g.:

- File: `cae9d182733848a7873983fe2b2264a0.pdf`
- Cert: `C94D3EFA-2493-4ED3-B9FC-96DD6754575F`

## Development

```bash
uv sync
uv run pytest -q
AVILOO_MCP_REPO_ROOT=/path/to/your/repo uv run aviloo-mcp
```
