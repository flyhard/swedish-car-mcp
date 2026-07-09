from __future__ import annotations

import os
from typing import Any

import httpx

from bilmarknad_mcp.schema import CarListing
from bilmarknad_mcp.soh import apply_soh

WAYKE_REST = "https://api.wayke.se/vehicles"
WAYKE_GQL = "https://gql.wayke.se/query"

GQL_SEARCH = """
query SearchVehicles($search: String, $skip: Int, $take: Int) {
  vehicles(search: $search, skip: $skip, take: $take) {
    id
    title
    shortDescription
    manufacturer { name }
    modelSeries { name }
    modelYear
    mileage
    price
    fuelType
    gearbox
    city
    organization { name }
    url
    image { url }
    registrationNumber
    published
  }
}
"""

GQL_VEHICLE = """
query GetVehicle($id: String!) {
  vehicle(id: $id) {
    id
    title
    shortDescription
    description
    manufacturer { name }
    modelSeries { name }
    modelYear
    mileage
    price
    fuelType
    gearbox
    city
    organization { name }
    url
    image { url }
    registrationNumber
    published
    data {
      properties
      options
    }
  }
}
"""


def _parse_vehicle(item: dict[str, Any]) -> CarListing:
    manufacturer = item.get("manufacturer") or {}
    series = item.get("modelSeries") or {}
    org = item.get("organization") or {}
    image = item.get("image") or {}
    mileage = item.get("mileage")
    price = item.get("price")
    listing = CarListing(
        source="wayke",
        id=str(item.get("id") or ""),
        title=item.get("title") or "",
        make=manufacturer.get("name"),
        model=series.get("name"),
        year=item.get("modelYear"),
        mileage_km=int(mileage) if mileage is not None else None,
        price_sek=int(price) if price is not None else None,
        fuel=item.get("fuelType"),
        transmission=item.get("gearbox"),
        location=item.get("city"),
        dealer_name=org.get("name"),
        url=item.get("url"),
        image_url=image.get("url"),
        published_at=item.get("published"),
        registration_number=item.get("registrationNumber"),
        raw=item,
    )
    apply_soh(listing, item.get("title"), item.get("shortDescription"), source="wayke_search")
    return listing

def _soh_detail_fields(item):
    fields = [item.get("description"), item.get("shortDescription")]
    data = item.get("data") or {}
    props = data.get("properties") or []
    opts = data.get("options") or []
    for coll in (props, opts):
        if isinstance(coll, list):
            for entry in coll:
                if isinstance(entry, dict):
                    fields.append(str(entry.get("name") or entry.get("label") or entry.get("value") or ""))
                else:
                    fields.append(str(entry))
        elif isinstance(coll, dict):
            for k, v in coll.items():
                fields.append(f"{k}: {v}")
    return [f for f in fields if f]


def _enrich_vehicle_soh(listing, item):
    apply_soh(listing, *_soh_detail_fields(item), source="wayke_detail")
    listing.raw = {**listing.raw, "detail": item}
    return listing


class WaykeClient:
    def __init__(self, client: httpx.Client | None = None, api_key: str | None = None):
        self._client = client
        self._owns = client is None
        self._api_key = api_key or os.environ.get("WAYKE_API_KEY")

    def _get_client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(timeout=30.0, headers={"User-Agent": "bilmarknad-mcp/0.1"})
        return self._client

    def search(
        self,
        q: str | None = None,
        rows: int = 20,
        page: int = 1,
    ) -> list[CarListing]:
        if self._api_key:
            rest = self._search_rest(q=q, rows=rows, page=page)
            if rest is not None:
                return rest
        return self._search_gql(q=q, rows=rows, page=page)

    def _search_rest(self, q, rows, page):
        client = self._get_client()
        headers = {"Authorization": f"Bearer {self._api_key}"}
        params: dict[str, Any] = {"take": rows, "skip": (page - 1) * rows}
        if q:
            params["search"] = q
        response = client.get(WAYKE_REST, params=params, headers=headers)
        if response.status_code == 401:
            return None
        response.raise_for_status()
        payload = response.json()
        items = payload if isinstance(payload, list) else payload.get("vehicles", payload.get("data", []))
        return [_parse_vehicle(item) for item in items]

    def _search_gql(self, q, rows, page):
        client = self._get_client()
        variables = {"search": q or "", "skip": (page - 1) * rows, "take": rows}
        response = client.post(
            WAYKE_GQL,
            json={"query": GQL_SEARCH, "variables": variables},
            headers={"Content-Type": "application/json"},
        )
        if response.status_code >= 400:
            return []
        data = response.json()
        vehicles = (((data.get("data") or {}).get("vehicles")) or [])
        return [_parse_vehicle(item) for item in vehicles]


    def get_vehicle(self, vehicle_id: str) -> CarListing | None:
        client = self._get_client()
        if self._api_key:
            response = client.get(f"{WAYKE_REST}/{vehicle_id}", headers={"Authorization": f"Bearer {self._api_key}"})
            if response.status_code == 200:
                item = response.json()
                listing = _parse_vehicle(item)
                return _enrich_vehicle_soh(listing, item)
        response = client.post(
            WAYKE_GQL,
            json={"query": GQL_VEHICLE, "variables": {"id": vehicle_id}},
            headers={"Content-Type": "application/json"},
        )
        if response.status_code >= 400:
            return None
        data = response.json()
        item = ((data.get("data") or {}).get("vehicle"))
        if not item:
            return None
        listing = _parse_vehicle(item)
        return _enrich_vehicle_soh(listing, item)

    def close(self):
        if self._owns and self._client is not None:
            self._client.close()

