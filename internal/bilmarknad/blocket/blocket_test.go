package blocket_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/blocket"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

func intPtr(v int) *int { return &v }

func TestBuildParamsMileageAndFuel(t *testing.T) {
	make := "KIA"
	params := blocket.BuildParams(blocket.SearchParams{
		Make: &make, Model: strPtr("Niro"), MileageToKM: intPtr(12000), Fuel: "el", Rows: 10, Page: 2,
	})
	if params["variant"] != "KIA/Niro" {
		t.Fatalf("variant = %q", params["variant"])
	}
	if params["mileage_to"] != "1200" {
		t.Fatalf("mileage_to = %q", params["mileage_to"])
	}
	if params["fuel"] != "4" {
		t.Fatalf("fuel = %q", params["fuel"])
	}
	if params["start"] != "10" || params["page"] != "2" {
		t.Fatalf("pagination = %v", params)
	}
}

func strPtr(s string) *string { return &s }

func TestParseAdMileageKM(t *testing.T) {
	listing := blocket.ParseAd(map[string]any{
		"ad_id":   float64(1),
		"heading": "Test",
		"mileage": float64(1500),
		"price":   map[string]any{"amount": float64(199000)},
	})
	if listing.ID != "1" {
		t.Fatalf("id = %q", listing.ID)
	}
	if listing.MileageKM == nil || *listing.MileageKM != 15000 {
		t.Fatalf("mileage_km = %v", listing.MileageKM)
	}
	if listing.PriceSEK == nil || *listing.PriceSEK != 199000 {
		t.Fatalf("price_sek = %v", listing.PriceSEK)
	}
}

func TestBlocketSearchMockTransport(t *testing.T) {
	client := blocket.NewClient(&http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		body := `{"docs":[{"ad_id":99,"heading":"Kia Niro","mileage":100,"price":{"amount":250000}}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})})
	defer client.Close()
	results, err := client.Search(context.Background(), blocket.SearchParams{Rows: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Kia Niro" {
		t.Fatalf("results = %+v", results)
	}
}

func TestNormalizeSourcesDefault(t *testing.T) {
	got := search.NormalizeSources(nil)
	want := []string{"blocket", "wayke", "kvd", "tradera", "riddermark", "carla"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
}

func TestListSourcesShape(t *testing.T) {
	data := (&search.Service{}).ListSources()
	if _, ok := data["sources"]; !ok {
		t.Fatal("missing sources")
	}
	if _, ok := data["env"]; !ok {
		t.Fatal("missing env")
	}
	ids := map[string]struct{}{}
	for _, s := range data["sources"].([]map[string]any) {
		ids[s["id"].(string)] = struct{}{}
	}
	for _, id := range []string{"blocket", "wayke", "kvd", "tradera", "riddermark", "carla"} {
		if _, ok := ids[id]; !ok {
			t.Fatalf("missing %s", id)
		}
	}
}

func TestURLBlocketIntegration(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.blocket.se/mobility/item/12345")
	if !ok || s != "blocket" || id != "12345" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestParseAdModelSpecificationSOH(t *testing.T) {
	listing := blocket.ParseAd(map[string]any{
		"ad_id": float64(42), "heading": "Kia e-Niro", "mileage": float64(100),
		"price": map[string]any{"amount": float64(250000)},
		"model_specification": "SOH 88% batterihälsa",
	})
	if listing.SOHPercent == nil || *listing.SOHPercent != 88.0 {
		t.Fatalf("soh_percent = %v", listing.SOHPercent)
	}
	if !listing.BatteryTested {
		t.Fatal("expected battery tested")
	}
	if listing.SOHSource == nil || *listing.SOHSource != "blocket_search" {
		t.Fatalf("soh_source = %v", listing.SOHSource)
	}
}
