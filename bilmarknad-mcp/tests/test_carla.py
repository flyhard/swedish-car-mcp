import json

import httpx

from bilmarknad_mcp.carla import CarlaClient, _map_fuel_type
from bilmarknad_mcp.search import SearchService
from bilmarknad_mcp.urls import parse_listing_url

CAR = {
    "objectID": "bmw-x1-2021-d4jbgei9io6g00fpicvg",
    "manufacturer": "BMW",
    "model": "X1",
    "version": "xDrive25e M-Sport",
    "modelYear": 2021,
    "mileage": 22000,
    "retailPrice": 329000,
    "type": "BEV",
    "registrationNumber": "SDM65X",
    "mainPhoto": "https://cdn.spinio.fi/carla/SDM65X/extra/outside07.jpg",
    "createdAt": "2026-01-08T15:02:54.312961+01:00",
}


def _search_html():
    payload = {
        "props": {
            "pageProps": {
                "initialResult": {
                    "totalHits": 1,
                    "cars": [CAR],
                }
            }
        }
    }
    return f'<html><script id="__NEXT_DATA__" type="application/json">{json.dumps(payload)}</script></html>'


def _detail_html():
    payload = {
        "props": {
            "pageProps": {
                "id": CAR["objectID"],
                "car": {
                    "manufacturer": "BMW",
                    "model": "X1",
                    "version": "xDrive25e M-Sport",
                    "registrationNumber": "SDM65X",
                    "mileage": 22000,
                    "modelYear": 2021,
                    "type": "BEV",
                    "transmission": "AUTOMATIC",
                    "price": {"retailPrice": 329000},
                    "outsidePhotos": [{"url": CAR["mainPhoto"]}],
                    "description": "92% batterihälsa",
                },
            }
        }
    }
    return f'<html><script id="__NEXT_DATA__" type="application/json">{json.dumps(payload)}</script></html>'


def test_map_carla_fuel_type():
    assert _map_fuel_type("el") == "BEV"
    assert _map_fuel_type("phev") == "PHEV"
    assert _map_fuel_type("laddhybrid") == "PHEV"


def test_carla_search_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=_search_html())

    transport = httpx.MockTransport(handler)
    client = CarlaClient(httpx.Client(transport=transport))
    try:
        results = client.search(make="BMW", fuel="el", rows=5)
    finally:
        client.close()
    assert len(results) == 1
    assert results[0].title.startswith("BMW X1")
    assert results[0].fuel == "El"


def test_carla_get_listing_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=_detail_html())

    transport = httpx.MockTransport(handler)
    client = CarlaClient(httpx.Client(transport=transport))
    try:
        listing = client.get_listing(CAR["objectID"])
    finally:
        client.close()
    assert listing is not None
    assert listing.id == CAR["objectID"]
    assert listing.soh_percent == 92.0


def test_url_carla():
    assert parse_listing_url("https://www.carla.se/bil/bmw-x1-2021-d4jbgei9io6g00fpicvg") == (
        "carla",
        "bmw-x1-2021-d4jbgei9io6g00fpicvg",
    )


def test_list_sources_includes_carla():
    data = SearchService().list_sources()
    ids = {s["id"] for s in data["sources"]}
    assert "carla" in ids
