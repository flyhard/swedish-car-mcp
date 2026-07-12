package carla_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/carla"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

var car = map[string]any{
	"objectID": "bmw-x1-2021-d4jbgei9io6g00fpicvg",
	"manufacturer": "BMW", "model": "X1", "version": "xDrive25e M-Sport",
	"modelYear": 2021, "mileage": 22000, "retailPrice": 329000, "type": "BEV",
	"registrationNumber": "SDM65X",
	"mainPhoto": "https://cdn.spinio.fi/carla/SDM65X/extra/outside07.jpg",
	"createdAt": "2026-01-08T15:02:54.312961+01:00",
}

func searchHTML() string {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"initialResult": map[string]any{"totalHits": 1, "cars": []any{car}},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return `<html><script id="__NEXT_DATA__" type="application/json">` + string(b) + `</script></html>`
}

func detailHTML() string {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"id": car["objectID"],
				"car": map[string]any{
					"manufacturer": "BMW", "model": "X1", "version": "xDrive25e M-Sport",
					"registrationNumber": "SDM65X", "mileage": 22000, "modelYear": 2021,
					"type": "BEV", "transmission": "AUTOMATIC",
					"price": map[string]any{"retailPrice": 329000},
					"outsidePhotos": []any{map[string]any{"url": car["mainPhoto"]}},
					"description": "92% batterihälsa",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return `<html><script id="__NEXT_DATA__" type="application/json">` + string(b) + `</script></html>`
}

func TestMapCarlaFuelType(t *testing.T) {
	if v := carla.MapFuelType("el"); v == nil || *v != "BEV" {
		t.Fatalf("el = %v", v)
	}
	if v := carla.MapFuelType("phev"); v == nil || *v != "PHEV" {
		t.Fatalf("phev = %v", v)
	}
}

func TestCarlaSearchMockTransport(t *testing.T) {
	client := carla.NewClient(&http.Client{Transport: rt(func(*http.Request) (*http.Response, error) {
		return textResp(200, searchHTML()), nil
	})})
	defer client.Close()
	make := "BMW"
	fuel := "el"
	results, err := client.Search(context.Background(), nil, &make, nil, &fuel, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !stringsHasPrefix(results[0].Title, "BMW X1") {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Fuel == nil || *results[0].Fuel != "El" {
		t.Fatalf("fuel = %v", results[0].Fuel)
	}
}

func TestCarlaGetListingMockTransport(t *testing.T) {
	client := carla.NewClient(&http.Client{Transport: rt(func(*http.Request) (*http.Response, error) {
		return textResp(200, detailHTML()), nil
	})})
	defer client.Close()
	listing, err := client.GetListing(context.Background(), car["objectID"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil || listing.ID != car["objectID"] {
		t.Fatalf("listing = %+v", listing)
	}
	if listing.SOHPercent == nil || *listing.SOHPercent != 92.0 {
		t.Fatalf("soh = %v", listing.SOHPercent)
	}
}

func TestListSourcesIncludesCarla(t *testing.T) {
	data := (&search.Service{}).ListSources()
	ids := map[string]struct{}{}
	for _, s := range data["sources"].([]map[string]any) {
		ids[s["id"].(string)] = struct{}{}
	}
	if _, ok := ids["carla"]; !ok {
		t.Fatal("missing carla")
	}
	_, _, ok := urls.ParseListingURL("https://www.carla.se/bil/bmw-x1-2021-d4jbgei9io6g00fpicvg")
	if !ok {
		t.Fatal("url parse failed")
	}
}

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func textResp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
