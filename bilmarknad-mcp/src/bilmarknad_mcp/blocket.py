from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

import httpx

from bilmarknad_mcp.schema import CarListing
from bilmarknad_mcp.soh import apply_soh

BLOCKET_SEARCH_URL = "https://www.blocket.se/mobility/search/api/search/SEARCH_ID_CAR_USED"
BLOCKET_DETAIL_URL = "https://blocket-api.se/v1/ad/car"


def pick(key, mapping, default=None):
    getter = getattr(mapping, "get")
    return getter(key, default)

FUEL_MAP = {"el": "4", "electric": "4", "bensin": "1", "diesel": "2"}
TRANSMISSION_MAP = {
    "manuell": "1",
    "manual": "1",
    "automat": "2",
    "automatic": "2",
    "automatisk": "2",
}


def _normalize_make(make):
    if not make:
        return None
    key = make.strip().upper().replace(" ", "_").replace("-", "_")
    if key in {"MERCEDES", "MERCEDES_BENZ"}:
        return "MERCEDES_BENZ"
    return key


def build_params(
    q=None,
    make=None,
    model=None,
    price_from=None,
    price_to=None,
    year_from=None,
    year_to=None,
    mileage_to_km=None,
    fuel=None,
    transmission=None,
    sort=None,
    rows=20,
    page=1,
):
    params = {}
    if q:
        params["q"] = q
    mk = _normalize_make(make)
    if mk and model:
        params["variant"] = f"{mk}/{model.strip()}"
    elif mk:
        params["make"] = mk
    elif model:
        params["q"] = model.strip()
    if price_from is not None:
        params["price_from"] = price_from
    if price_to is not None:
        params["price_to"] = price_to
    if year_from is not None:
        params["year_from"] = year_from
    if year_to is not None:
        params["year_to"] = year_to
    if mileage_to_km is not None:
        params["mileage_to"] = max(1, int(mileage_to_km / 10))
    if fuel is not None:
        if isinstance(fuel, int) or str(fuel).isdigit():
            params["fuel"] = str(fuel)
        else:
            params["fuel"] = pick(str(fuel).lower(), FUEL_MAP, str(fuel))
    if transmission is not None:
        if isinstance(transmission, int) or str(transmission).isdigit():
            params["transmission"] = str(transmission)
        else:
            params["transmission"] = pick(
                str(transmission).lower(), TRANSMISSION_MAP, str(transmission)
            )
    if sort:
        params["sort"] = sort
    params["rows"] = rows
    start = (page - 1) * rows
    if start:
        params["start"] = start
        params["page"] = page
    return params

def parse_ad(p):
    mileage = pick("mileage", p)
    mileage_km = int(mileage) * 10 if mileage is not None else None
    price = pick("price", p) or {}
    amount = pick("amount", price)
    image = pick("image", p) or {}
    img_url = pick("url", image)
    url_list = pick("image_urls", p, list())
    ts = pick("timestamp", p)
    published = None
    if ts:
        published = datetime.fromtimestamp(ts / 1000, tz=timezone.utc).isoformat()
    listing_id = str(pick("ad_id", p) or pick("id", p) or "")
    listing = CarListing(
        source="blocket",
        id=listing_id,
        title=pick("heading", p) or pick("facade_title", p) or "",
        make=pick("make", p),
        model=pick("model", p),
        year=pick("year", p),
        mileage_km=mileage_km,
        price_sek=int(amount) if amount is not None else None,
        fuel=pick("fuel", p),
        transmission=pick("transmission", p),
        location=pick("location", p),
        dealer_name=pick("organisation_name", p) or pick("dealer_segment", p),
        url=pick("canonical_url", p),
        image_url=img_url or (url_list[0] if url_list else None),
        published_at=published,
        registration_number=pick("regno", p),
        raw=p,
    )

    extras = pick("extras", p, [])
    labels = pick("labels", p, [])
    extra_texts = [str(x) for x in extras] if isinstance(extras, list) else []
    label_texts = [str(x) for x in labels] if isinstance(labels, list) else []

    apply_soh(
        listing,
        pick("model_specification", p),
        pick("heading", p),
        *extra_texts,
        *label_texts,
        source="blocket_search",
    )
    return listing


def fetch_blocket_detail(client, listing_id):
    response = client.get(BLOCKET_DETAIL_URL, params={"id": listing_id})
    response.raise_for_status()
    return response.json()

def enrich_listing_with_detail(listing, client):
    if not listing.id:
        return listing
    try:
        detail = fetch_blocket_detail(client, listing.id)
    except Exception:
        return listing
    equipment = detail.get("equipment") or []
    equip_texts = []
    for item in equipment:
        if isinstance(item, dict):
            equip_texts.append(str(item.get("name") or item.get("label") or item))
        else:
            equip_texts.append(str(item))
    apply_soh(
        listing,
        detail.get("subtitle"),
        *equip_texts,
        source="blocket_detail",
    )
    listing.raw = {**listing.raw, "detail": detail}
    return listing
class BlocketClient:
    def __init__(self, client=None):
        self._client = client
        self._owns = client is None

    def _get_client(self):
        if self._client is None:
            self._client = httpx.Client(timeout=30.0, headers={"User-Agent": "bilmarknad-mcp/0.1"})
        return self._client

    def search(self, **kwargs):
        params = build_params(**kwargs)
        client = self._get_client()
        response = client.get(BLOCKET_SEARCH_URL, params=params)
        response.raise_for_status()
        data = response.json()
        return [parse_ad(item) for item in pick("docs", data, [])]

    def get_listing(self, listing_id):
        for item in self.search(q=listing_id, rows=60):
            if item.id is not None and str(item.id) == str(listing_id):
                return enrich_listing_with_detail(item, self._get_client())
        return None

    def close(self):
        if self._owns and self._client is not None:
            self._client.close()
