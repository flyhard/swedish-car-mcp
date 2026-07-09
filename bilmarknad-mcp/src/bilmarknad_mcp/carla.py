from __future__ import annotations

import json
import re
from typing import Any
from urllib.parse import quote, urlencode

import httpx

from bilmarknad_mcp.schema import CarListing
from bilmarknad_mcp.soh import apply_soh

CARLA_BASE = "https://www.carla.se"
SEARCH_PATH = "/kopa-bil"
_DEFAULT_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
        "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
    ),
    "Accept-Language": "sv-SE,sv;q=0.9",
}
_NEXT_DATA_RE = re.compile(
    r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>',
    re.DOTALL,
)


def _fuel_label(car_type: str | None) -> str | None:
    if not car_type:
        return None
    mapping = {
        "BEV": "El",
        "PHEV": "Laddhybrid",
        "EV": "El",
    }
    return mapping.get(car_type.upper(), car_type)


def _transmission_label(value: str | None) -> str | None:
    if not value:
        return None
    if value.upper() == "AUTOMATIC":
        return "Automat"
    if value.upper() == "MANUAL":
        return "Manuell"
    return value


def _title(item: dict[str, Any]) -> str:
    parts = [item.get("manufacturer"), item.get("model"), item.get("version")]
    return " ".join(part for part in parts if part)


def _listing_url(object_id: str) -> str:
    slug = object_id.strip().strip("/")
    return f"{CARLA_BASE}/bil/{slug}"


def _detail_fields(item: dict[str, Any]) -> list[str | None]:
    return [
        _title(item),
        item.get("version"),
        item.get("batteryTestResult"),
        json.dumps(item.get("batteryState"), ensure_ascii=False) if item.get("batteryState") else None,
    ]


def _parse_search_car(item: dict[str, Any]) -> CarListing:
    object_id = str(item.get("objectID") or "")
    listing = CarListing(
        source="carla",
        id=object_id,
        title=_title(item),
        make=item.get("manufacturer"),
        model=item.get("model"),
        year=item.get("modelYear"),
        mileage_km=item.get("mileage"),
        price_sek=item.get("retailPrice"),
        fuel=_fuel_label(item.get("type")),
        transmission=None,
        location=None,
        dealer_name="Carla",
        url=_listing_url(object_id),
        image_url=item.get("mainPhoto"),
        published_at=item.get("createdAt"),
        registration_number=item.get("registrationNumber"),
        raw=item,
    )
    apply_soh(listing, _title(item), source="carla_search")
    return listing


def _parse_detail_car(item: dict[str, Any], object_id: str) -> CarListing:
    price = item.get("price") or {}
    listing = CarListing(
        source="carla",
        id=object_id,
        title=_title(item),
        make=item.get("manufacturer"),
        model=item.get("model"),
        year=item.get("modelYear"),
        mileage_km=item.get("mileage"),
        price_sek=price.get("retailPrice"),
        fuel=_fuel_label(item.get("type")),
        transmission=_transmission_label(item.get("transmission")),
        location=None,
        dealer_name="Carla",
        url=_listing_url(object_id),
        image_url=((item.get("outsidePhotos") or [{}])[0] or {}).get("url")
        if item.get("outsidePhotos")
        else None,
        published_at=None,
        registration_number=item.get("registrationNumber"),
        raw=item,
    )
    apply_soh(listing, *_detail_fields(item), source="carla_detail")
    return listing


def _extract_next_data(html: str) -> dict[str, Any]:
    match = _NEXT_DATA_RE.search(html)
    if not match:
        return {}
    return json.loads(match.group(1))


def _map_fuel_type(fuel: str | None) -> str | None:
    if not fuel:
        return None
    key = fuel.strip().lower()
    if key in {"el", "ev", "electric", "elektrisk", "bev"}:
        return "BEV"
    if key in {"phev", "laddhybrid", "plugin", "plug-in"}:
        return "PHEV"
    return None


class CarlaClient:
    def __init__(self, client: httpx.Client | None = None):
        self._client = client
        self._owns = client is None

    def _get_client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(
                timeout=30.0,
                headers=_DEFAULT_HEADERS,
                follow_redirects=True,
            )
        return self._client

    def _fetch_next_data(self, path: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
        query = f"?{urlencode(params, doseq=True)}" if params else ""
        response = self._get_client().get(f"{CARLA_BASE}{path}{query}")
        response.raise_for_status()
        return _extract_next_data(response.text)

    def search(
        self,
        q: str | None = None,
        make: str | None = None,
        model: str | None = None,
        fuel: str | None = None,
        rows: int = 20,
        page: int = 1,
    ) -> list[CarListing]:
        params: dict[str, Any] = {"page": page}
        if q:
            params["search"] = q
        elif make and model:
            params["manufacturerAndModel"] = f"{make} {model}"
        elif make:
            params["manufacturer"] = make

        fuel_type = _map_fuel_type(fuel)
        if fuel_type:
            params["type"] = fuel_type

        data = self._fetch_next_data(SEARCH_PATH, params)
        result = ((data.get("props") or {}).get("pageProps") or {}).get("initialResult") or {}
        cars = result.get("cars") or []
        listings = [_parse_search_car(item) for item in cars if isinstance(item, dict)]
        return listings[:rows]

    def get_listing(self, listing_id: str) -> CarListing | None:
        slug = (listing_id or "").strip().strip("/")
        if not slug:
            return None
        if slug.startswith("bil/"):
            slug = slug.split("/", 1)[1]

        data = self._fetch_next_data(f"/bil/{quote(slug, safe='-')}")
        car = ((data.get("props") or {}).get("pageProps") or {}).get("car")
        if not isinstance(car, dict):
            return None
        page_id = ((data.get("props") or {}).get("pageProps") or {}).get("id") or slug
        return _parse_detail_car(car, str(page_id))

    def close(self) -> None:
        if self._owns and self._client is not None:
            self._client.close()
