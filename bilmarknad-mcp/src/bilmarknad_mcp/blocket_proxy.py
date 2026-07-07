from __future__ import annotations

import os
from typing import Any

import httpx

from bilmarknad_mcp.blocket import parse_ad
from bilmarknad_mcp.schema import CarListing

DEFAULT_PROXY_BASE = "https://blocket-api.se"


def proxy_base_url() -> str:
    return (os.environ.get("BLOCKET_PROXY_URL") or DEFAULT_PROXY_BASE).rstrip("/")


class BlocketProxyClient:
    def __init__(self, client: httpx.Client | None = None, base_url: str | None = None):
        self._client = client
        self._owns = client is None
        self._base_url = (base_url or proxy_base_url()).rstrip("/")

    def _get_client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(timeout=30.0, headers={"User-Agent": "bilmarknad-mcp/0.1"})
        return self._client

    def search(self, **kwargs: Any) -> list[CarListing]:
        params = dict(kwargs)
        client = self._get_client()
        response = client.get(f"{self._base_url}/search", params=params)
        if response.status_code >= 400:
            return []
        payload = response.json()
        docs = payload.get("docs") or payload.get("results") or payload
        if not isinstance(docs, list):
            return []
        return [parse_ad(item) for item in docs]

    def close(self) -> None:
        if self._owns and self._client is not None:
            self._client.close()
