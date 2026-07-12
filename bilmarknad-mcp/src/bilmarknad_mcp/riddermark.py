from __future__ import annotations

import json
import re
from typing import Any
from urllib.parse import quote, urlencode

import httpx

from bilmarknad_mcp.schema import CarListing
from bilmarknad_mcp.soh import apply_soh

RIDDERMARK_BASE = "https://www.riddermarkbil.se"
SEARCH_PATH = "/kopa-bil/"
_NEXT_DATA_RE = re.compile(
    r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>',
    re.DOTALL,
)


def _make_slug(make: str) -> str:
    return quote((make or "").strip().lower(), safe="")


def _listing_id(make: str, license_plate: str) -> str:
    return f"{_make_slug(make)}/{license_plate.strip().lower()}"


def _listing_url(make: str, license_plate: str) -> str:
    return f"{RIDDERMARK_BASE}/kopa-bil/{_listing_id(make, license_plate)}/"


def _cover_image(item: dict[str, Any]) -> str | None:
    cover = item.get("coverImage")
    if isinstance(cover, dict) and cover.get("url"):
        return cover["url"]
    images = item.get("images") or []
    if images and isinstance(images[0], dict):
        return images[0].get("url")
    return None


def _location_name(item: dict[str, Any]) -> str | None:
    location = item.get("location") or item.get("physicalLocation") or {}
    if isinstance(location, dict):
        return location.get("name")
    return None


def _parse_car(item: dict[str, Any], *, detail: bool = False) -> CarListing:
    make = item.get("make") or ""
    license_plate = item.get("licenseplate") or ""
    listing = CarListing(
        source="riddermark",
        id=_listing_id(make, license_plate),
        title=item.get("title") or item.get("carName") or "",
        make=make or None,
        model=item.get("model") or None,
        year=item.get("modelYear"),
        mileage_km=item.get("mileage"),
        price_sek=item.get("price"),
        fuel=item.get("fuelType"),
        transmission=item.get("gearboxType"),
        location=_location_name(item),
        dealer_name="Riddermark Bil",
        url=_listing_url(make, license_plate),
        image_url=_cover_image(item),
        published_at=item.get("publishedAt"),
        registration_number=license_plate or None,
        raw=item,
    )
    source = "riddermark_detail" if detail else "riddermark_search"
    apply_soh(
        listing,
        item.get("title"),
        item.get("modelDescription"),
        item.get("fullText"),
        source=source,
    )
    return listing


def _extract_next_data(html: str) -> dict[str, Any]:
    match = _NEXT_DATA_RE.search(html)
    if not match:
        return {}
    return json.loads(match.group(1))


class RiddermarkClient:
    def __init__(self, client: httpx.Client | None = None):
        self._client = client
        self._owns = client is None

    def _get_client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(
                timeout=30.0,
                headers={"User-Agent": "bilmarknad-mcp/0.1"},
                follow_redirects=True,
            )
        return self._client

    def _fetch_next_data(self, path: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
        query = f"?{urlencode(params, doseq=True)}" if params else ""
        response = self._get_client().get(f"{RIDDERMARK_BASE}{path}{query}")
        response.raise_for_status()
        return _extract_next_data(response.text)

    def search(
        self,
        q: str | None = None,
        make: str | None = None,
        model: str | None = None,
        price_min: int | None = None,
        price_max: int | None = None,
        mileage_max_km: int | None = None,
        rows: int = 20,
        page: int = 1,
    ) -> list[CarListing]:
        params: dict[str, Any] = {"page": page}
        search_parts = [part for part in (q, make, model) if part]
        if search_parts:
            params["search"] = " ".join(search_parts)
        if price_min is not None:
            params["priceFrom"] = price_min
        if price_max is not None:
            params["priceTo"] = price_max
        if mileage_max_km is not None:
            params["mileageTo"] = mileage_max_km

        data = self._fetch_next_data(SEARCH_PATH, params)
        cars = ((data.get("props") or {}).get("pageProps") or {}).get("carsJson") or []
        listings = [_parse_car(item) for item in cars if isinstance(item, dict)]
        return listings[:rows]

    def get_listing(self, listing_id: str) -> CarListing | None:
        listing_id = (listing_id or "").strip().strip("/")
        if not listing_id:
            return None

        if "/" in listing_id:
            make_slug, license_plate = listing_id.split("/", 1)
            path = f"/kopa-bil/{make_slug}/{license_plate}/"
            data = self._fetch_next_data(path)
            advert = ((data.get("props") or {}).get("pageProps") or {}).get("advertJson")
            if isinstance(advert, dict):
                return _parse_car(advert, detail=True)
            return None

        results = self.search(q=listing_id, rows=5, page=1)
        target = listing_id.lower()
        for item in results:
            if item.registration_number and item.registration_number.lower() == target:
                return self.get_listing(item.id)
            if item.id.split("/", 1)[-1] == target:
                return self.get_listing(item.id)
        return None

    def close(self) -> None:
        if self._owns and self._client is not None:
            self._client.close()


def parse_riddermark_url(url):
    import importlib
    mod=importlib.import_module("bilmarknad_mcp.urls")
    parsed=mod.parse_listing_url(url)
    return parsed if parsed and parsed[0]=="riddermark" else None
