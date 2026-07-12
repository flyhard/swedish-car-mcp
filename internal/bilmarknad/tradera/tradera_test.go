package tradera_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/tradera"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

const searchXML = `<?xml version="1.0" encoding="utf-8"?>
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
</SearchResult>`

const itemXML = `<?xml version="1.0" encoding="utf-8"?>
<Item xmlns="http://api.tradera.com">
  <Id>123456</Id>
  <ShortDescription>Kia e-Niro 2021</ShortDescription>
  <LongDescription>99% SoH batteritestad med Aviloo</LongDescription>
  <BuyItNowPrice>249000</BuyItNowPrice>
  <SellerAlias>bilhandlare1</SellerAlias>
  <SellerCity>Stockholm</SellerCity>
  <ItemUrl>https://www.tradera.com/item/123456</ItemUrl>
</Item>`

func TestParseXMLResponseSearch(t *testing.T) {
	parsed, err := tradera.ParseXMLResponse(searchXML)
	if err != nil {
		t.Fatal(err)
	}
	sr, ok := parsed["SearchResult"].(map[string]any)
	if !ok {
		t.Fatalf("SearchResult = %T", parsed["SearchResult"])
	}
	items, ok := sr["Items"].(map[string]any)
	if !ok {
		t.Fatalf("Items = %T", sr["Items"])
	}
	item, ok := items["Item"].(map[string]any)
	if !ok {
		t.Fatalf("Item = %T", items["Item"])
	}
	if item["Id"] != "123456" {
		t.Fatalf("Id = %v", item["Id"])
	}
}

func TestParseTraderaItemPriceAndSOH(t *testing.T) {
	parsed, err := tradera.ParseXMLResponse(searchXML)
	if err != nil {
		t.Fatal(err)
	}
	sr := parsed["SearchResult"].(map[string]any)
	rawItem := sr["Items"].(map[string]any)["Item"].(map[string]any)
	listing := tradera.ParseTraderaItem(rawItem, false)
	if listing.Source != "tradera" || listing.ID != "123456" {
		t.Fatalf("listing = %+v", listing)
	}
	if listing.Title != "Kia e-Niro 2021" {
		t.Fatalf("title = %q", listing.Title)
	}
	if listing.PriceSEK == nil || *listing.PriceSEK != 249000 {
		t.Fatalf("price = %v", listing.PriceSEK)
	}
	if listing.Location == nil || *listing.Location != "Stockholm" {
		t.Fatalf("location = %v", listing.Location)
	}
	if listing.SOHPercent == nil || *listing.SOHPercent != 99.0 {
		t.Fatalf("soh = %v", listing.SOHPercent)
	}
	if !listing.BatteryTested {
		t.Fatal("expected battery tested")
	}
}

func TestTraderaSearchMockTransport(t *testing.T) {
	client := tradera.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "Search") {
			return resp(200, searchXML), nil
		}
		return resp(404, ""), nil
	})}, "", "", 0)
	defer client.Close()
	q := "Kia Niro"
	results, err := client.Search(context.Background(), &q, 5, 1, "Relevance")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Kia e-Niro 2021" {
		t.Fatalf("results = %+v", results)
	}
}

func TestTraderaGetListingMockTransport(t *testing.T) {
	client := tradera.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "GetItem") {
			return resp(200, itemXML), nil
		}
		return resp(404, ""), nil
	})}, "", "", 0)
	defer client.Close()
	listing, err := client.GetListing(context.Background(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil || listing.ID != "123456" {
		t.Fatalf("listing = %+v", listing)
	}
	if listing.SOHPercent == nil || *listing.SOHPercent != 99.0 {
		t.Fatalf("soh = %v", listing.SOHPercent)
	}
}

func TestListSourcesIncludesTradera(t *testing.T) {
	data := (&search.Service{}).ListSources()
	ids := map[string]struct{}{}
	for _, s := range data["sources"].([]map[string]any) {
		ids[s["id"].(string)] = struct{}{}
	}
	if _, ok := ids["tradera"]; !ok {
		t.Fatal("missing tradera")
	}
	env := data["env"].(map[string]any)
	if _, ok := env["TRADERA_APP_ID"]; !ok {
		t.Fatal("missing TRADERA_APP_ID env")
	}
	_, _, ok := urls.ParseListingURL("https://www.tradera.com/item/123456")
	if !ok {
		t.Fatal("url parse failed")
	}
}

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func resp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}
