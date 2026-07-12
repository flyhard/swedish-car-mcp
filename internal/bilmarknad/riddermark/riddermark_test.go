package riddermark_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/riddermark"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

var car = map[string]any{
	"id": 12345, "make": "Kia", "model": "Niro", "modelYear": 2021,
	"mileage": 15000, "price": 249000, "fuelType": "El", "licenseplate": "ABC123",
	"title": "Kia Niro EV", "modelDescription": "95% SoH batteritestad",
	"location": map[string]any{"name": "Stockholm"},
	"coverImage": map[string]any{"url": "https://ride.blob.core.windows.net/car-images/test.jpg"},
}

func html(pageProps map[string]any) string {
	payload := map[string]any{"props": map[string]any{"pageProps": pageProps}}
	b, _ := json.Marshal(payload)
	return `<html><script id="__NEXT_DATA__" type="application/json">` + string(b) + `</script></html>`
}

func TestRiddermarkSearchMockTransport(t *testing.T) {
	client := riddermark.NewClient(&http.Client{Transport: rt(func(*http.Request) (*http.Response, error) {
		return textResp(200, html(map[string]any{"carsJson": []any{car}})), nil
	})})
	defer client.Close()
	q := "Kia"
	results, err := client.Search(context.Background(), &q, nil, nil, nil, nil, nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Kia Niro EV" || results[0].ID != "kia/abc123" {
		t.Fatalf("results = %+v", results)
	}
}

func TestRiddermarkGetListingMockTransport(t *testing.T) {
	client := riddermark.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/kopa-bil/kia/abc123/") {
			return textResp(200, html(map[string]any{"advertJson": car})), nil
		}
		return textResp(200, html(map[string]any{"carsJson": []any{car}})), nil
	})})
	defer client.Close()
	listing, err := client.GetListing(context.Background(), "kia/abc123")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil || listing.ID != "kia/abc123" {
		t.Fatalf("listing = %+v", listing)
	}
	if listing.SOHPercent == nil || *listing.SOHPercent != 95.0 {
		t.Fatalf("soh = %v", listing.SOHPercent)
	}
}

func TestListSourcesIncludesRiddermark(t *testing.T) {
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
	_, _, ok := urls.ParseListingURL("https://www.riddermarkbil.se/kopa-bil/kia/ABC123/")
	if !ok {
		t.Fatal("url parse failed")
	}
}

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func textResp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}
