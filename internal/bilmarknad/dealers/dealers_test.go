package dealers_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/carla"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/riddermark"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

const riddermarkSearchHTML = `<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {"pageProps": {"carsJson": [{
    "id": 311149, "make": "Kia", "model": "E-Niro", "modelDescription": "64 kWh 204hk",
    "title": "Kia E-Niro 64 kWh 204hk", "carName": "Kia e-Niro 64 kWh, 204hk, 2022",
    "licenseplate": "ZGH41H", "modelYear": 2022, "mileage": 8755, "price": 238700,
    "fuelType": "El", "gearboxType": "Automatisk", "publishedAt": "2026-07-06T15:24:38.3543295",
    "location": {"name": "Recharge Stockholm"},
    "coverImage": {"url": "https://ride.blob.core.windows.net/car-images/test.jpg"}
  }]}}
}</script></body></html>`

const carlaSearchHTML = `<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {"pageProps": {"initialResult": {"totalHits": 1, "cars": [{
    "objectID": "bmw-x1-2021-d4jbgei9io6g00fpicvg",
    "manufacturer": "BMW", "model": "X1", "version": "xDrive25e M-Sport",
    "registrationNumber": "SDM65X", "mileage": 99126, "modelYear": 2021,
    "retailPrice": 262990, "type": "PHEV",
    "mainPhoto": "https://cdn.spinio.fi/carla/SDM65X/extra/outside07.jpg",
    "createdAt": "2026-01-08T15:02:54.312961+01:00"
  }]}}}
}</script></body></html>`

const carlaDetailHTML = `<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {"pageProps": {
    "id": "bmw-x1-2021-d4jbgei9io6g00fpicvg",
    "car": {
      "manufacturer": "BMW", "model": "X1", "version": "xDrive25e M-Sport",
      "registrationNumber": "SDM65X", "mileage": 99126, "modelYear": 2021,
      "type": "PHEV", "transmission": "AUTOMATIC",
      "price": {"retailPrice": 260490, "listPrice": 270490, "discount": 10000},
      "outsidePhotos": [{"url": "https://cdn.spinio.fi/carla/SDM65X/extra/outside07.jpg"}]
    }
  }}
}</script></body></html>`

func TestURLDealers(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.riddermarkbil.se/kopa-bil/kia/zgh41h/")
	if !ok || s != "riddermark" || id != "kia/zgh41h" {
		t.Fatalf("riddermark url: %q %q %v", s, id, ok)
	}
	s, id, ok = urls.ParseListingURL("https://www.carla.se/bil/bmw-x1-2021-d4jbgei9io6g00fpicvg")
	if !ok || s != "carla" || id != "bmw-x1-2021-d4jbgei9io6g00fpicvg" {
		t.Fatalf("carla url: %q %q %v", s, id, ok)
	}
}

func TestRiddermarkDealerSearch(t *testing.T) {
	client := riddermark.NewClient(&http.Client{Transport: rt(func(*http.Request) (*http.Response, error) {
		return textResp(200, riddermarkSearchHTML), nil
	})})
	defer client.Close()
	q := "kia"
	results, err := client.Search(context.Background(), &q, nil, nil, nil, nil, nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Source != "riddermark" || results[0].ID != "kia/zgh41h" {
		t.Fatalf("results = %+v", results)
	}
	if results[0].PriceSEK == nil || *results[0].PriceSEK != 238700 {
		t.Fatalf("price = %v", results[0].PriceSEK)
	}
}

func TestCarlaDealerSearch(t *testing.T) {
	client := carla.NewClient(&http.Client{Transport: rt(func(*http.Request) (*http.Response, error) {
		return textResp(200, carlaSearchHTML), nil
	})})
	defer client.Close()
	make := "BMW"
	results, err := client.Search(context.Background(), nil, &make, nil, nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Source != "carla" {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Fuel == nil || *results[0].Fuel != "Laddhybrid" {
		t.Fatalf("fuel = %v", results[0].Fuel)
	}
}

func TestCarlaDealerGetListing(t *testing.T) {
	client := carla.NewClient(&http.Client{Transport: rt(func(*http.Request) (*http.Response, error) {
		return textResp(200, carlaDetailHTML), nil
	})})
	defer client.Close()
	item, err := client.GetListing(context.Background(), "bmw-x1-2021-d4jbgei9io6g00fpicvg")
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.PriceSEK == nil || *item.PriceSEK != 260490 {
		t.Fatalf("price = %v", item)
	}
	if item.Transmission == nil || *item.Transmission != "Automat" {
		t.Fatalf("transmission = %v", item.Transmission)
	}
}

func TestListSourcesIncludesDealers(t *testing.T) {
	data := (&search.Service{}).ListSources()
	ids := map[string]struct{}{}
	for _, s := range data["sources"].([]map[string]any) {
		ids[s["id"].(string)] = struct{}{}
	}
	for _, id := range []string{"riddermark", "carla"} {
		if _, ok := ids[id]; !ok {
			t.Fatalf("missing %s", id)
		}
	}
}

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func textResp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}
