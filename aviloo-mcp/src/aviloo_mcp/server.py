from __future__ import annotations

import json
from pathlib import Path

from mcp.server.fastmcp import FastMCP

from aviloo_mcp.parser import extract_text, parse_aviloo_text
from aviloo_mcp.repo import assert_in_repo, find_cert_in_repo, find_pdfs, repo_root

mcp = FastMCP("aviloo")



def _rel(p: Path) -> str:
    try:
        return str(p.relative_to(repo_root()))
    except ValueError:
        return str(p)


@mcp.tool()
def extract_aviloo_pdf(pdf_path: str) -> str:
    root = repo_root()
    candidate = Path(pdf_path)
    if not candidate.is_absolute():
        candidate = root / candidate
    resolved = assert_in_repo(candidate)
    if not resolved.is_file():
        raise FileNotFoundError(_rel(resolved))
    text = extract_text(resolved)
    data = parse_aviloo_text(text, download_id=resolved.stem)
    data["pdf_path"] = _rel(resolved)
    return json.dumps(data, ensure_ascii=False, indent=2)


@mcp.tool()
def lookup_aviloo_cert(cert_or_id: str) -> str:
    found = find_cert_in_repo(cert_or_id)
    if found is None:
        return json.dumps({"found": False, "cert_or_id": cert_or_id}, ensure_ascii=False, indent=2)
    text = extract_text(found)
    data = parse_aviloo_text(text, download_id=found.stem)
    data["found"] = True
    data["pdf_path"] = _rel(found)
    return json.dumps(data, ensure_ascii=False, indent=2)


@mcp.tool()
def list_aviloo_pdfs() -> str:
    pdfs = find_pdfs()
    payload = {
        "repo_root": str(repo_root()),
        "count": len(pdfs),
        "pdfs": [_rel(p) for p in pdfs],
    }
    return json.dumps(payload, ensure_ascii=False, indent=2)


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()
