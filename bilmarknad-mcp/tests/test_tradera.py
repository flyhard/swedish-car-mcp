import httpx

from bilmarknad_mcp.search import SearchService, _normalize_sources
from bilmarknad_mcp.tradera import (
    TraderaClient,
    parse_tradera_item,
    parse_xml_response,
)
from bilmarknad_mcp.urls import parse_listing_url

SEARCH_XML = """<?xml version="1.0" encoding="utf-8"?>
<SearchResult xmlns="http://api.tradera.com">
  <Items>
    <Item>
      <Id>123456</Id>
      <ShortDescription>Kia e-Niro 2021</ShortDescription>
      <CategoryId>10</CategoryId>
      <SellerAlias>bilhandlare1</SellerAlias>
      <SellerCity>Stockholm</SellerCity>
      <BuyItNowPrice>249000</BuyItNowPrice>
      <StartDate>2026-01-01T10:00:00</StartDate>
      <ThumbnailLink>https://img.tradera.net/images/1.jpg</ThumbnailLink>
      <ItemUrl>https://www.tradera.com/item/123456</ItemUrl>
      <LongDescription>99% SoH batteritestad med Aviloo</LongDescription>
    </Item>
  </Items>
</SearchResult>
"""

ITEM_XML = """<?xml version="1.0" encoding="utf-8"?>
<Item xmlns="http://api.tradera.com">
  <Id>123456</Id>
  <ShortDescription>Kia e-Niro 2021</ShortDescription>
  <LongDescription>99% SoH batteritestad med Aviloo</LongDescription>
  <BuyItNowPrice>249000</BuyItNowPrice>
  <SellerAlias>bilhandlare1</SellerAlias>
  <SellerCity>Stockholm</SellerCity>
  <ItemUrl>https://www.tradera.com/item/123456</ItemUrl>
</Item>
"""


def test_parse_xml_response_search():
    parsed = parse_xml_response(SEARCH_XML)
    assert "SearchResult" in parsed
    items = parsed["SearchResult"]["Items"]
    assert items["Item"]["Id"] == "123456"


def test_parse_tradera_item_price_and_soh():
    parsed = parse_xml_response(SEARCH_XML)
    raw_item = parsed["SearchResult"]["Items"]["Item"]
    listing = parse_tradera_item(raw_item)
    assert listing.source == "tradera"
    assert listing.id == "123456"
    assert listing.title == "Kia e-Niro 2021"
    assert listing.price_sek == 249000
    assert listing.location == "Stockholm"
    assert listing.soh_percent == 99.0
    assert listing.battery_tested is True


def test_tradera_search_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        if "Search" in str(request.url):
            return httpx.Response(200, text=SEARCH_XML)
        return httpx.Response(404)

    transport = httpx.MockTransport(handler)
    client = TraderaClient(httpx.Client(transport=transport))
    try:
        results = client.search(q="Kia Niro", rows=5)
    finally:
        client.close()
    assert len(results) == 1
    assert results[0].title == "Kia e-Niro 2021"


def test_tradera_get_listing_mock_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        if "GetItem" in str(request.url):
            return httpx.Response(200, text=ITEM_XML)
        return httpx.Response(404)

    transport = httpx.MockTransport(handler)
    client = TraderaClient(httpx.Client(transport=transport))
    try:
        listing = client.get_listing("123456")
    finally:
        client.close()
    assert listing is not None
    assert listing.id == "123456"
    assert listing.soh_percent == 99.0


def test_url_tradera():
    assert parse_listing_url("https://www.tradera.com/item/123456") == ("tradera", "123456")
    assert parse_listing_url("https://www.tradera.se/item/99") == ("tradera", "99")


def test_normalize_sources_includes_tradera():
    assert _normalize_sources(None) == ["blocket", "wayke", "kvd", "tradera", "riddermark", "carla"]


def test_list_sources_includes_tradera():
    data = SearchService().list_sources()
    ids = {s["id"] for s in data["sources"]}
    assert "tradera" in ids
    assert "TRADERA_APP_ID" in data["env"]
