import httpx

from bilmarknad_mcp.carla import CarlaClient
from bilmarknad_mcp.search import SearchService, _normalize_sources
from bilmarknad_mcp.urls import parse_listing_url


RIDDERMARK_SEARCH_HTML = """
<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {
    "pageProps": {
      "carsJson": [{
        "id": 311149,
        "make": "Kia",
        "model": "E-Niro",
        "modelDescription": "64 kWh 204hk",
        "title": "Kia E-Niro 64 kWh 204hk",
        "carName": "Kia e-Niro 64 kWh, 204hk, 2022",
        "licenseplate": "ZGH41H",
        "modelYear": 2022,
        "mileage": 8755,
        "price": 238700,
        "fuelType": "El",
        "gearboxType": "Automatisk",
        "publishedAt": "2026-07-06T15:24:38.3543295",
        "location": {"name": "Recharge Stockholm"},
        "coverImage": {"url": "https://ride.blob.core.windows.net/car-images/test.jpg"}
      }]
    }
  }
}</script>
</body></html>
"""

CARLA_SEARCH_HTML = """
<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {
    "pageProps": {
      "initialResult": {
        "totalHits": 1,
        "cars": [{
          "objectID": "bmw-x1-2021-d4jbgei9io6g00fpicvg",
          "manufacturer": "BMW",
          "model": "X1",
          "version": "xDrive25e M-Sport",
          "registrationNumber": "SDM65X",
          "mileage": 99126,
          "modelYear": 2021,
          "retailPrice": 262990,
          "type": "PHEV",
          "mainPhoto": "https://cdn.spinio.fi/carla/SDM65X/extra/outside07.jpg",
          "createdAt": "2026-01-08T15:02:54.312961+01:00"
        }]
      }
    }
  }
}</script>
</body></html>
"""

CARLA_DETAIL_HTML = """
<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {
    "pageProps": {
      "id": "bmw-x1-2021-d4jbgei9io6g00fpicvg",
      "car": {
        "manufacturer": "BMW",
        "model": "X1",
        "version": "xDrive25e M-Sport",
        "registrationNumber": "SDM65X",
        "mileage": 99126,
        "modelYear": 2021,
        "type": "PHEV",
        "transmission": "AUTOMATIC",
        "price": {"retailPrice": 260490, "listPrice": 270490, "discount": 10000},
        "outsidePhotos": [{"url": "https://cdn.spinio.fi/carla/SDM65X/extra/outside07.jpg"}]
      }
    }
  }
}</script>
</body></html>
"""


def test_url_riddermark():
    assert parse_listing_url("https://www.riddermarkbil.se/kopa-bil/kia/zgh41h/") == (
        "riddermark",
        "kia/zgh41h",
    )


def test_url_carla():
    assert parse_listing_url("https://www.carla.se/bil/bmw-x1-2021-d4jbgei9io6g00fpicvg") == (
        "carla",
        "bmw-x1-2021-d4jbgei9io6g00fpicvg",
    )


def test_normalize_sources_includes_dealers():
    assert _normalize_sources(None) == [
        "blocket",
        "wayke",
        "kvd",
        "tradera",
        "riddermark",
        "carla",
    ]


def test_riddermark_search_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=RIDDERMARK_SEARCH_HTML)

    from bilmarknad_mcp.riddermark import RiddermarkClient

    client = RiddermarkClient(httpx.Client(transport=httpx.MockTransport(handler)))
    try:
        results = client.search(q="kia", rows=5)
    finally:
        client.close()
    assert len(results) == 1
    assert results[0].source == "riddermark"
    assert results[0].id == "kia/zgh41h"
    assert results[0].price_sek == 238700


def test_carla_search_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=CARLA_SEARCH_HTML)

    client = CarlaClient(httpx.Client(transport=httpx.MockTransport(handler)))
    try:
        results = client.search(make="BMW", rows=5)
    finally:
        client.close()
    assert len(results) == 1
    assert results[0].source == "carla"
    assert results[0].id == "bmw-x1-2021-d4jbgei9io6g00fpicvg"
    assert results[0].fuel == "Laddhybrid"


def test_carla_get_listing_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=CARLA_DETAIL_HTML)

    client = CarlaClient(httpx.Client(transport=httpx.MockTransport(handler)))
    try:
        item = client.get_listing("bmw-x1-2021-d4jbgei9io6g00fpicvg")
    finally:
        client.close()
    assert item is not None
    assert item.price_sek == 260490
    assert item.transmission == "Automat"


def test_list_sources_includes_dealers():
    data = SearchService().list_sources()
    ids = {source["id"] for source in data["sources"]}
    assert {"riddermark", "carla"}.issubset(ids)
