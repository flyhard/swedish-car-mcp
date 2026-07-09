from __future__ import annotations

import os
import time
import xml.etree.ElementTree as ET
from typing import Any

import httpx

from bilmarknad_mcp.schema import CarListing
from bilmarknad_mcp.soh import apply_soh

API_BASE = "https://api.tradera.com/v3"
SEARCH_URL = f"{API_BASE}/searchservice.asmx/Search"
GET_ITEM_URL = f"{API_BASE}/publicservice.asmx/GetItem"

# Fordon > Bilar on Tradera (override with TRADERA_CAR_CATEGORY_ID).
DEFAULT_CAR_CATEGORY_ID = 10

CACHE_TTL_SEARCH_SEC = 30 * 60
CACHE_TTL_ITEM_SEC = 30 * 60


class TraderaUnavailableError(RuntimeError):
    """Raised when Tradera cannot be reached or credentials are missing."""


def _local_tag(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def _text(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def _to_int(value: Any) -> int | None:
    if value is None or value == "":
        return None
    try:
        return int(float(str(value)))
    except (TypeError, ValueError):
        return None


def _to_float(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(str(value))
    except (TypeError, ValueError):
        return None


def _as_list(value: Any) -> list[Any]:
    if value is None:
        return []
    if isinstance(value, list):
        return value
    return [value]


def etree_to_obj(elem: ET.Element) -> Any:
    children = list(elem)
    if not children:
        text = (elem.text or "").strip()
        return text if text else None
    result: dict[str, Any] = {}
    for child in children:
        key = _local_tag(child.tag)
        value = etree_to_obj(child)
        if key in result:
            existing = result[key]
            if isinstance(existing, list):
                existing.append(value)
            else:
                result[key] = [existing, value]
        else:
            result[key] = value
    return result


def parse_xml_response(text: str) -> dict[str, Any]:
    root = ET.fromstring(text)
    obj = etree_to_obj(root)
    return {_local_tag(root.tag): obj}


def _dig(data: dict[str, Any], *keys: str) -> Any:
    current: Any = data
    for key in keys:
        if not isinstance(current, dict):
            return None
        matched = None
        for candidate, value in current.items():
            if _local_tag(candidate) == key or candidate == key:
                matched = value
                break
        current = matched
    return current


def _parse_attributes(attr_values: Any) -> dict[str, str]:
    attrs: dict[str, str] = {}
    term_values = _dig(attr_values if isinstance(attr_values, dict) else {}, "TermAttributeValues", "TermAttributeValue")
    for entry in _as_list(term_values):
        if not isinstance(entry, dict):
            continue
        name = _text(entry.get("Name"))
        values = entry.get("Values")
        value = None
        if isinstance(values, dict):
            value = _text(values.get("string"))
        elif isinstance(values, list):
            value = _text(values[0]) if values else None
        else:
            value = _text(values)
        if not name or not value:
            continue
        key = name.lower()
        if key in {"condition", "skick"}:
            attrs["condition"] = value
        elif key in {"mobile_brand", "märke", "brand", "make"}:
            attrs["brand"] = value
        elif key in {"mobile_model", "modell", "model"}:
            attrs["model"] = value
    return attrs


def _parse_image_urls(image_links: Any) -> list[str]:
    urls: list[str] = []
    for entry in _as_list(image_links):
        if isinstance(entry, dict):
            url = _text(entry.get("Url") or entry.get("url"))
        else:
            url = _text(entry)
        if url:
            urls.append(url)
    return urls


def _parse_item(item: dict[str, Any], detailed: bool = False) -> dict[str, Any]:
    attrs = _parse_attributes(item.get("AttributeValues"))
    seller = item.get("Seller") if isinstance(item.get("Seller"), dict) else {}
    parsed = {
        "item_id": _to_int(item.get("Id") or item.get("ItemId")) or 0,
        "short_description": _text(item.get("ShortDescription") or item.get("Title")) or "",
        "long_description": _text(item.get("LongDescription")),
        "category_id": _to_int(item.get("CategoryId")) or 0,
        "category_name": _text(item.get("CategoryName")),
        "seller_id": _to_int(item.get("SellerId")) or 0,
        "seller_alias": _text(item.get("SellerAlias")),
        "seller_city": _text(item.get("SellerCity") or seller.get("City")),
        "start_price": _to_int(item.get("StartPrice")),
        "buy_it_now_price": _to_int(item.get("BuyItNowPrice")),
        "current_bid": _to_int(item.get("MaxBid") or item.get("CurrentBid")),
        "bid_count": _to_int(item.get("TotalBids")) or 0,
        "start_date": _text(item.get("StartDate")),
        "end_date": _text(item.get("EndDate")),
        "thumbnail_url": _text(item.get("ThumbnailLink")),
        "image_urls": _parse_image_urls(item.get("ImageLinks")),
        "item_type": _text(item.get("ItemType")) or "Auction",
        "item_url": _text(item.get("ItemUrl")),
        "attributes": attrs,
    }
    if detailed:
        parsed["seller_rating"] = _to_float(item.get("SellerDsrAverage") or seller.get("TotalRating"))
    return parsed


def _price_sek(item: dict[str, Any]) -> int | None:
    for key in ("buy_it_now_price", "current_bid", "start_price"):
        value = item.get(key)
        if value is not None and value > 0:
            return int(value)
    return None


def parse_tradera_item(raw: dict[str, Any], detailed: bool = False) -> CarListing:
    item = _parse_item(raw, detailed=detailed)
    attrs = item.get("attributes") or {}
    image_urls = item.get("image_urls") or []
    thumbnail = item.get("thumbnail_url")
    listing = CarListing(
        source="tradera",
        id=str(item["item_id"]),
        title=item["short_description"],
        make=attrs.get("brand"),
        model=attrs.get("model"),
        price_sek=_price_sek(item),
        location=item.get("seller_city"),
        dealer_name=item.get("seller_alias"),
        url=item.get("item_url") or f"https://www.tradera.com/item/{item['item_id']}",
        image_url=thumbnail or (image_urls[0] if image_urls else None),
        published_at=item.get("start_date"),
        raw=item,
    )
    apply_soh(
        listing,
        item.get("short_description"),
        item.get("long_description"),
        source="tradera_detail" if detailed else "tradera_search",
    )
    return listing


def _extract_search_items(parsed: dict[str, Any]) -> list[dict[str, Any]]:
    items = _dig(parsed, "SearchResult", "Items")
    if items is None:
        items = _dig(parsed, "Items")
    if items is None:
        return []
    if isinstance(items, dict):
        if any(key in items for key in ("Id", "ItemId", "ShortDescription", "Title")):
            return [items]
        nested = items.get("Item")
        if nested is not None:
            return [entry for entry in _as_list(nested) if isinstance(entry, dict)]
    return [entry for entry in _as_list(items) if isinstance(entry, dict)]


def _extract_item(parsed: dict[str, Any]) -> dict[str, Any] | None:
    item = _dig(parsed, "Item")
    if isinstance(item, dict):
        return item
    for value in parsed.values():
        if isinstance(value, dict) and any(key in value for key in ("Id", "ItemId", "ShortDescription", "Title")):
            return value
    return None


class _TimedCache:
    def __init__(self):
        self._entries: dict[str, tuple[float, Any]] = {}

    def get(self, key: str, ttl_sec: int) -> Any | None:
        entry = self._entries.get(key)
        if entry is None:
            return None
        stored_at, value = entry
        if time.monotonic() - stored_at > ttl_sec:
            self._entries.pop(key, None)
            return None
        return value

    def set(self, key: str, value: Any) -> None:
        self._entries[key] = (time.monotonic(), value)


class TraderaClient:
    def __init__(
        self,
        client: httpx.Client | None = None,
        app_id: str | None = None,
        app_key: str | None = None,
        car_category_id: int | None = None,
    ):
        self._client = client
        self._owns = client is None
        self._app_id = app_id or os.environ.get("TRADERA_APP_ID", "5572")
        self._app_key = app_key or os.environ.get("TRADERA_APP_KEY", "81974dd3-404d-456e-b050-b030ba646d6a")
        raw_category = car_category_id
        if raw_category is None:
            env_category = os.environ.get("TRADERA_CAR_CATEGORY_ID")
            raw_category = int(env_category) if env_category else DEFAULT_CAR_CATEGORY_ID
        self._car_category_id = raw_category
        self._cache = _TimedCache()

    def _get_client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(timeout=30.0, headers={"User-Agent": "bilmarknad-mcp/0.2"})
        return self._client

    def _auth_params(self) -> dict[str, str]:
        if not self._app_id or not self._app_key:
            raise TraderaUnavailableError("TRADERA_APP_ID and TRADERA_APP_KEY are required")
        return {"appId": str(self._app_id), "appKey": str(self._app_key)}

    def _get_xml(self, url: str, params: dict[str, Any]) -> dict[str, Any]:
        client = self._get_client()
        response = client.get(url, params={**self._auth_params(), **params})
        if response.status_code >= 400:
            raise TraderaUnavailableError(f"Tradera API returned {response.status_code}")
        return parse_xml_response(response.text)

    def search(
        self,
        q: str | None = None,
        rows: int = 20,
        page: int = 1,
        order_by: str = "Relevance",
        category_id: int | None = None,
    ) -> list[CarListing]:
        query = (q or "").strip() or "bil"
        category = self._car_category_id if category_id is None else category_id
        cache_key = f"search:{query}:{category}:{order_by}:{rows}:{page}"
        cached = self._cache.get(cache_key, CACHE_TTL_SEARCH_SEC)
        if cached is not None:
            return cached

        parsed = self._get_xml(
            SEARCH_URL,
            {
                "query": query,
                "categoryId": str(category),
                "orderBy": order_by,
                "pageNumber": str(page),
            },
        )
        listings = [parse_tradera_item(item) for item in _extract_search_items(parsed)]
        self._cache.set(cache_key, listings[:rows])
        return listings[:rows]

    def get_listing(self, listing_id: str) -> CarListing | None:
        cache_key = f"item:{listing_id}"
        cached = self._cache.get(cache_key, CACHE_TTL_ITEM_SEC)
        if cached is not None:
            return cached

        parsed = self._get_xml(GET_ITEM_URL, {"itemId": str(listing_id)})
        raw_item = _extract_item(parsed)
        if not raw_item:
            return None
        listing = parse_tradera_item(raw_item, detailed=True)
        self._cache.set(cache_key, listing)
        return listing

    def close(self) -> None:
        if self._owns and self._client is not None:
            self._client.close()
