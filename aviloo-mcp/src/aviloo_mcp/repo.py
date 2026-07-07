"""Resolve and constrain paths to the configured project repository."""

from __future__ import annotations

import os
import re
from pathlib import Path

_HEX32 = re.compile(r"^[0-9a-fA-F]{32}$")
_UUID = re.compile(
    r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"
)

_PACKAGE_DIR = Path(__file__).resolve().parent
_DEFAULT_REPO_ROOT = _PACKAGE_DIR.parent.parent.parent.parent


def repo_root() -> Path:
    env = os.environ.get("AVILOO_MCP_REPO_ROOT")
    if env:
        return Path(env).resolve()
    return _DEFAULT_REPO_ROOT.resolve()


def assert_in_repo(path: Path) -> Path:
    """Resolve path and ensure it stays under REPO_ROOT."""
    root = repo_root()
    resolved = path.resolve()
    try:
        resolved.relative_to(root)
    except ValueError as exc:
        raise ValueError(f"Path is outside repository: {resolved}") from exc
    return resolved


def _looks_like_aviloo_pdf(path: Path) -> bool:
    name_lower = path.name.lower()
    if "aviloo" in name_lower:
        return True
    if _HEX32.match(path.stem):
        return True
    parts = path.parts
    if "docs" in parts and path.suffix.lower() == ".pdf":
        return True
    if path.parent.resolve() == repo_root() and path.suffix.lower() == ".pdf":
        return True
    return False


def find_pdfs() -> list[Path]:
    """List PDF paths in the repo that may be AVILOO certificates."""
    root = repo_root()
    if not root.is_dir():
        return []
    candidates: list[Path] = []
    for path in root.rglob("*.pdf"):
        if not path.is_file():
            continue
        try:
            assert_in_repo(path)
        except ValueError:
            continue
        if _looks_like_aviloo_pdf(path):
            candidates.append(path)
    return sorted(candidates, key=lambda p: str(p.relative_to(root)))


def find_cert_in_repo(cert_or_id: str) -> Path | None:
    """Find PDF by certificate UUID or 32-char hex download id (filename stem)."""
    key = cert_or_id.strip()
    key_lower = key.lower()
    root = repo_root()

    if _HEX32.match(key):
        for pdf in find_pdfs():
            if pdf.stem.lower() == key_lower:
                return pdf
        direct = root / f"{key_lower}.pdf"
        if direct.is_file():
            return assert_in_repo(direct)
        for pdf in root.rglob(f"{key_lower}.pdf"):
            try:
                return assert_in_repo(pdf)
            except ValueError:
                continue

    if _UUID.match(key):
        from aviloo_mcp.parser import extract_text, parse_aviloo_text

        target = key.upper()
        for pdf in find_pdfs():
            if pdf.stem.upper() == target:
                return pdf
        for pdf in find_pdfs():
            try:
                text = extract_text(pdf)
                parsed = parse_aviloo_text(text, download_id=pdf.stem)
                cert = parsed.get("certificate_number")
                if cert and cert.upper() == target:
                    return pdf
            except Exception:
                continue
    return None
