import json

import httpx

from bilmarknad_mcp.riddermark import RiddermarkClient
from bilmarknad_mcp.search import SearchService, _normalize_sources
from bilmarknad_mcp.urls import parse_listing_url

CAR = {
    "id": 12345,
    "make": "Kia",
    "model": "Niro",
    "modelYear": 2021,
    "mileage": 15000,
    "price": 249000,
    "fuelType": "El",
    "licenseplate": "ABC123",
    "title": "Kia Niro EV",
    "modelDescription": "95% SoH batteritestad",
    "location": {"name": "Stockholm"},
    "coverImage": {"url": "https://ride.blob.core.windows.net/car-images/test.jpg"},
}


def _html(page_props):
    payload = {"props": {"pageProps": page_props}}
    return f'<html><script id="__NEXT_DATA__" type="application/json">{json.dumps(payload)}</script></html>'


def test_riddermark_search_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=_html({"carsJson": [CAR]}))

    transport = httpx.MockTransport(handler)
    client = RiddermarkClient(httpx.Client(transport=transport))
    try:
        results = client.search(q="Kia", rows=5)
    finally:
        client.close()
    assert len(results) == 1
    assert results[0].title == "Kia Niro EV"
    assert results[0].id == "kia/abc123"


def test_riddermark_get_listing_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        if "/kopa-bil/kia/abc123/" in str(request.url):
            return httpx.Response(200, text=_html({"advertJson": CAR}))
        return httpx.Response(200, text=_html({"carsJson": [CAR]}))

    transport = httpx.MockTransport(handler)
    client = RiddermarkClient(httpx.Client(transport=transport))
    try:
        listing = client.get_listing("kia/abc123")
    finally:
        client.close()
    assert listing is not None
    assert listing.id == "kia/abc123"
    assert listing.soh_percent == 95.0


def test_url_riddermark():
    assert parse_listing_url("https://www.riddermarkbil.se/kopa-bil/kia/ABC123/") == (
        "riddermark",
        "kia/abc123",
    )


def test_normalize_sources_includes_riddermark():
    assert "riddermark" in _normalize_sources(None)
    assert "carla" in _normalize_sources(None)


def test_list_sources_includes_riddermark():
    data = SearchService().list_sources()
    ids = {s["id"] for s in data["sources"]}
    assert "riddermark" in ids
    assert "carla" in ids
