package wayke_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/wayke"
)

func searchHTML() string {
	payload := `{"_id":"0fe35f55-7142-4665-aad4-e853128d735e","acceptsTestDriveInquiries":false,"title":"Volvo V90 Cross Country","manufacturer":"Volvo","modelSeries":"V90","modelYear":2018,"mileage":12253,"odometerReading":{"unit":"ScandinavianMile","value":12253},"price":299800,"fuelType":"Diesel","gearboxType":"Automat","shortDescription":"D4 AWD Momentum Plus","itemPublished":"2026-07-12T18:14:20.8482534Z","position":{"city":"Strängnäs"},"branches":[{"name":"Riddermark Bil"}],"featuredImage":{"files":[{"formats":[{"format":"770x514","url":"https://cdn.wayke.se/example.jpg"}]}]}}`
	return `<html><body>"documents":[` + payload + `]</body></html>`
}

func detailHTML() string {
	return `<html><head><script type="application/ld+json">{
	  "@type":"Car",
	  "name":"Volvo V90 Cross Country",
	  "brand":{"name":"Volvo"},
	  "model":"V90",
	  "vehicleModelDate":"2018",
	  "vehicleConfiguration":"D4 AWD",
	  "vehicleTransmission":"Automat",
	  "identifier":{"propertyID":"registrationNumber","value":"RMC648"},
	  "mileageFromOdometer":{"value":"122530","unitCode":"KMT"},
	  "vehicleEngine":{"fuelType":"Diesel"},
	  "image":["https://cdn.wayke.se/detail.jpg"],
	  "offers":{"price":299800,"validFrom":"2026-07-12T18:14:20.77+00:00","seller":{"name":"Riddermark Bil"}}
	}</script></head></html>`
}

func searchHTMLWithPlate() string {
	payload := `{"_id":"0fe35f55-7142-4665-aad4-e853128d735e","acceptsTestDriveInquiries":false,"title":"Volvo V90 Cross Country","manufacturer":"Volvo","modelSeries":"V90","modelYear":2018,"mileage":12253,"registrationNumber":"RMC648","price":299800,"fuelType":"Diesel","gearboxType":"Automat","shortDescription":"D4 AWD","itemPublished":"2026-07-12T18:14:20.8482534Z","position":{"city":"Strängnäs"},"branches":[{"name":"Riddermark Bil"}],"featuredImage":{"files":[{"formats":[{"format":"770x514","url":"https://cdn.wayke.se/example.jpg"}]}]}}`
	return `<html><body>"documents":[` + payload + `]</body></html>`
}

func TestWaykeGetByLicensePlateMockTransport(t *testing.T) {
	client := wayke.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/sok") {
			return textResp(200, searchHTMLWithPlate()), nil
		}
		if strings.Contains(req.URL.Path, "/objekt/0fe35f55-7142-4665-aad4-e853128d735e") {
			return textResp(200, detailHTML()), nil
		}
		return textResp(404, ""), nil
	})}, "")
	defer client.Close()

	listing, err := client.GetByLicensePlate(context.Background(), "rmc648")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil || listing.RegistrationNumber == nil || *listing.RegistrationNumber != "RMC648" {
		t.Fatalf("listing = %+v", listing)
	}
}

func TestWaykeSearchScrapeMockTransport(t *testing.T) {
	client := wayke.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/sok/volvo") {
			return textResp(200, searchHTML()), nil
		}
		return textResp(404, ""), nil
	})}, "")
	defer client.Close()

	q := "volvo"
	results, err := client.Search(context.Background(), &q, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].ID != "0fe35f55-7142-4665-aad4-e853128d735e" {
		t.Fatalf("id = %q", results[0].ID)
	}
	if results[0].MileageKM == nil || *results[0].MileageKM != 122530 {
		t.Fatalf("mileage = %v", results[0].MileageKM)
	}
}

func TestWaykeGetVehicleScrapeMockTransport(t *testing.T) {
	client := wayke.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/objekt/0fe35f55-7142-4665-aad4-e853128d735e") {
			return textResp(200, detailHTML()), nil
		}
		return textResp(404, ""), nil
	})}, "")
	defer client.Close()

	listing, err := client.GetVehicle(context.Background(), "0fe35f55-7142-4665-aad4-e853128d735e")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil || listing.RegistrationNumber == nil || *listing.RegistrationNumber != "RMC648" {
		t.Fatalf("listing = %+v", listing)
	}
}

func TestURLWaykeObjekt(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.wayke.se/objekt/0fe35f55-7142-4665-aad4-e853128d735e")
	if !ok || s != "wayke" || id != "0fe35f55-7142-4665-aad4-e853128d735e" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func textResp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}
