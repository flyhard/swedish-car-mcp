package ayvens_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/ayvens"
)

func TestAyvensGetListingEnrichesSOHFromBUS(t *testing.T) {
	const (
		tid = "88081b5f-4a2f-443e-987f-02ecd687d616"
		ts  = "secret-token"
	)
	detail := strings.Replace(detailHTML, "tid=test", "tid="+tid+"&ts="+ts, 1)

	accessToken := makeTestAccessToken(t, map[string]any{
		"salesReportId": float64(2882),
		"testId":        float64(36571),
	})

	client := ayvens.NewClient(&http.Client{Transport: rt(func(req *http.Request) (*http.Response, error) {
		u := req.URL.String()
		switch {
		case strings.Contains(req.URL.Path, "tmbjc7ny2pf043314-skoda-enyaq.html"):
			return textResp(200, detail), nil
		case strings.Contains(u, "SecurityPortal"):
			return jsonResp(200, map[string]any{
				"data": map[string]any{
					"tokens": map[string]any{"accessToken": accessToken},
				},
			}), nil
		case strings.Contains(u, "BUStest.Server"):
			body := readBody(req)
			switch {
			case strings.Contains(body, "getBatteryTestMetaData"):
				return jsonResp(200, map[string]any{
					"data": map[string]any{
						"batteryTestMetaData": map[string]any{
							"batteryResult":                     96,
							"isBatteryReadingExcludeFromReport": false,
						},
					},
				}), nil
			case strings.Contains(body, "getAvilooBatteryScore"):
				return jsonResp(200, map[string]any{"data": map[string]any{"avilooBatteryScore": 96}}), nil
			case strings.Contains(body, "getAvilooReportForTest"):
				return jsonResp(200, map[string]any{
					"data": map[string]any{
						"avilooReport": map[string]any{
							"sasUrl": "https://example.test/AvilooReport.pdf",
						},
					},
				}), nil
			default:
				return jsonResp(400, map[string]any{"errors": []any{map[string]any{"message": "unexpected query"}}}), nil
			}
		default:
			return textResp(404, ""), nil
		}
	})})
	defer client.Close()

	listing, err := client.GetListing(context.Background(), "koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil {
		t.Fatal("nil listing")
	}
	if listing.SOHPercent == nil || *listing.SOHPercent != 96 {
		t.Fatalf("soh = %v", listing.SOHPercent)
	}
	if !listing.BatteryTested {
		t.Fatal("expected battery tested")
	}
	if listing.SOHSource == nil || *listing.SOHSource != "ayvens_bus" {
		t.Fatalf("soh source = %v", listing.SOHSource)
	}
	if listing.Raw["aviloo_report_url"] == nil {
		t.Fatal("missing aviloo report url")
	}
	if listing.Raw["inspection_test_id"] != 36571 {
		t.Fatalf("test id = %v", listing.Raw["inspection_test_id"])
	}
}

func makeTestAccessToken(t *testing.T, business map[string]any) string {
	t.Helper()
	wrapper, _ := json.Marshal(map[string]any{
		"name":  "BUSPlatform3-Business Token",
		"id":    "test",
		"value": mustJSON(business),
	})
	payload, _ := json.Marshal(map[string]any{"businessToken": string(wrapper)})
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func readBody(req *http.Request) string {
	if req.Body == nil {
		return ""
	}
	b, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(strings.NewReader(string(b)))
	return string(b)
}

func jsonResp(code int, v any) *http.Response {
	b, _ := json.Marshal(v)
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(string(b)))}
}
