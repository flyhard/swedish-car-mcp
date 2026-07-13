package ayvens_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/ayvens"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

const searchHTML = `<html><body>
<div class="product" data-pid="tmbjc7ny2pf043314-skoda-enyaq">
<span data-tracking-productclick="[{&quot;event&quot;:&quot;productClick&quot;,&quot;ecommerce&quot;:{&quot;click&quot;:{&quot;products&quot;:[{&quot;id&quot;:&quot;tmbjc7ny2pf043314-skoda-enyaq&quot;,&quot;name&quot;:&quot;Enyaq&quot;,&quot;price&quot;:309000,&quot;brand&quot;:&quot;Skoda&quot;,&quot;variant&quot;:&quot;204hk iV80&quot;,&quot;dimension2&quot;:&quot;Electric&quot;,&quot;dimension3&quot;:&quot;Automatic&quot;,&quot;dimension5&quot;:11725,&quot;dimension56&quot;:&quot;2023-07-11&quot;}]}}}]"></span>
<a href="/sv-se/koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq.html">Skoda Enyaq</a>
</div>
<script type="application/ld+json">{"@context":"http://schema.org/","@type":"ItemList","itemListElement":[{"@type":"ListItem","position":1,"url":"https://usedcars.ayvens.com/sv-se/koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq.html"}]}</script>
</body></html>`

const detailHTML = `<html><body>
<h1 class="product-name">Skoda Enyaq</h1>
<p class="product-description">204hk iV80</p>
<span class="value" content="309000.00">309 000 kr</span>
<div class="detail-container registrationDate"><span class="value ml-0">2023-07-11</span></div>
<div class="detail-container fuelType"><span class="value ml-0">Elektrisk</span></div>
<div class="detail-container mileage"><span class="value ml-0">11&nbsp;725 mil</span></div>
<div class="detail-container gearType"><span class="value ml-0">Automatisk</span></div>
<div class="detail-container licensePlate"><span class="value ml-0">XUT95E</span></div>
<div class="detail-container location"><span class="value ml-0">Upplands Vasby</span></div>
<a href="https://bus2.bus.no/BUSPlatform3/BUStest.Client/#/salesReportLink?tid=test" class="d-none inspection-report-url"></a>
<img src="https://usedcars.ayvens.com/dw/image/v2/BFQV_PRD/example.jpg" />
</body></html>`

func TestAyvensSearchMockTransport(t *testing.T) {
	client := ayvens.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "Search-Show") {
			return textResp(200, searchHTML), nil
		}
		return textResp(404, ""), nil
	})})
	defer client.Close()
	results, err := client.Search(context.Background(), nil, nil, nil, nil, nil, nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d", len(results))
	}
	if results[0].ID != "tmbjc7ny2pf043314-skoda-enyaq" {
		t.Fatalf("id = %q", results[0].ID)
	}
	if results[0].PriceSEK == nil || *results[0].PriceSEK != 309000 {
		t.Fatalf("price = %v", results[0].PriceSEK)
	}
	if results[0].MileageKM == nil || *results[0].MileageKM != 117250 {
		t.Fatalf("mileage = %v", results[0].MileageKM)
	}
	if results[0].URL == nil || !strings.Contains(*results[0].URL, "tmbjc7ny2pf043314-skoda-enyaq") {
		t.Fatalf("url = %v", results[0].URL)
	}
}

func TestAyvensGetListingMockTransport(t *testing.T) {
	client := ayvens.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "tmbjc7ny2pf043314-skoda-enyaq.html") {
			return textResp(200, detailHTML), nil
		}
		return textResp(404, ""), nil
	})})
	defer client.Close()
	listing, err := client.GetListing(context.Background(), "koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil || listing.RegistrationNumber == nil || *listing.RegistrationNumber != "XUT95E" {
		t.Fatalf("listing = %+v", listing)
	}
	if listing.Raw["inspection_report_url"] == nil {
		t.Fatal("missing inspection report url")
	}
}

func TestListSourcesIncludesAyvens(t *testing.T) {
	data := (&search.Service{}).ListSources()
	ids := map[string]struct{}{}
	for _, s := range data["sources"].([]map[string]any) {
		ids[s["id"].(string)] = struct{}{}
	}
	if _, ok := ids["ayvens"]; !ok {
		t.Fatal("missing ayvens")
	}
}

func TestURLAyvens(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://usedcars.ayvens.com/sv-se/koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq.html")
	if !ok || s != "ayvens" || id != "tmbjc7ny2pf043314-skoda-enyaq" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func textResp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}
