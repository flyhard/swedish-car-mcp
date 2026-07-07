from __future__ import annotations

from typing import Any

import httpx

from bilmarknad_mcp.schema import CarListing

KVD_BASE = "https://www.kvd.se"
CANDIDATE_PATHS = (
    "/api/vehicles",
    "/api/search",
    "/api/v1/vehicles",
)


class KvdUnavailableError(RuntimeError):
    pass


class KvdClient:
    def __init__(self, client: httpx.Client | None = None):
        self._client = client
        self._owns = client is None
        self._available: bool | None = None

    def _get_client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(timeout=20.0, headers={"User-Agent": "bilmarknad-mcp/0.1"})
        return self._client

    def probe(self) -> bool:
        if self._available is not None:
            return self._available
        client = self._get_client()
        for path in CANDIDATE_PATHS:
            try:
                response = client.get(KVD_BASE + path, params={"limit": 1})
            except httpx.HTTPError:
                continue
            if response.status_code == 200 and "application/json" in response.headers.get(
                "content-type", ""
            ):
                self._available = True
                return True
        self._available = False
        return False

    def search(self, **kwargs: Any) -> list[CarListing]:
        if not self.probe():
            raise KvdUnavailableError("No public KVD API endpoint responded with JSON")
        return []

    def close(self) -> None:
        if self._owns and self._client is not None:
            self._client.close()

