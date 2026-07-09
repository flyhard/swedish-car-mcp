import httpx

from bilmarknad_mcp.blocket import BlocketClient, build_params, parse_ad
from bilmarknad_mcp.search import SearchService, _normalize_sources
from bilmarknad_mcp.urls import parse_listing_url


def test_build_params_mileage_and_fuel():
    params = build_params(make="KIA", model="Niro", mileage_to_km=12000, fuel="el", rows=10, page=2)
    assert params["variant"] == "KIA/Niro"
    assert params["mileage_to"] == 1200
    assert params["fuel"] == "4"
    assert params["rows"] == 10
    assert params["start"] == 10
    assert params["page"] == 2


def test_parse_ad_mileage_km():
    listing = parse_ad({"ad_id": 1, "heading": "Test", "mileage": 1500, "price": {"amount": 199000}})
    assert listing.id == "1"
    assert listing.mileage_km == 15000
    assert listing.price_sek == 199000


def test_blocket_search_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={"docs": [{"ad_id": 99, "heading": "Kia Niro", "mileage": 100, "price": {"amount": 250000}}]},
        )

    transport = httpx.MockTransport(handler)
    client = BlocketClient(httpx.Client(transport=transport))
    try:
        results = client.search(rows=1)
    finally:
        client.close()
    assert len(results) == 1
    assert results[0].title == "Kia Niro"


def test_url_blocket():
    assert parse_listing_url("https://www.blocket.se/mobility/item/12345") == ("blocket", "12345")


def test_normalize_sources_default():
    assert _normalize_sources(None) == ["blocket", "wayke", "kvd", "tradera", "riddermark", "carla"]


def test_list_sources_shape():
    data = SearchService().list_sources()
    assert "sources" in data and "env" in data
    ids = {s["id"] for s in data["sources"]}
    assert ids == {"blocket", "wayke", "kvd", "tradera", "riddermark", "carla"}
