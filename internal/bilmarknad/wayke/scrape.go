package wayke

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

const siteURL = "https://www.wayke.se"

var (
	vehicleDocRE = regexp.MustCompile(`\{"_id":"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}","acceptsTestDriveInquiries"`)
	jsonLDRE     = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
)

func (c *Client) searchScrape(ctx context.Context, q *string, rows, page int) ([]schema.CarListing, error) {
	if rows <= 0 {
		rows = 20
	}
	if page <= 0 {
		page = 1
	}
	params := url.Values{}
	params.Set("hits", fmt.Sprint(rows))
	if page > 1 {
		params.Set("offset", fmt.Sprint((page-1)*rows))
	}
	html, err := c.fetchHTML(ctx, searchPath(q), params)
	if err != nil {
		return nil, err
	}
	docs := extractDocuments(html)
	out := make([]schema.CarListing, 0, len(docs))
	for _, item := range docs {
		out = append(out, parseScrapedVehicle(item))
	}
	if len(out) > rows {
		out = out[:rows]
	}
	return out, nil
}

func (c *Client) getVehicleScrape(ctx context.Context, vehicleID string) (*schema.CarListing, error) {
	vehicleID = strings.Trim(strings.TrimSpace(vehicleID), "/")
	if vehicleID == "" {
		return nil, nil
	}
	html, err := c.fetchHTML(ctx, "/objekt/"+vehicleID, nil)
	if err != nil {
		return nil, err
	}
	for _, block := range jsonLDRE.FindAllStringSubmatch(html, -1) {
		var data map[string]any
		if json.Unmarshal([]byte(block[1]), &data) != nil {
			continue
		}
		if fmt.Sprint(data["@type"]) != "Car" {
			continue
		}
		listing := parseJSONLDCar(data, vehicleID)
		soh.Apply(&listing, "wayke_detail", detailFieldsFromJSONLD(data)...)
		return &listing, nil
	}
	return nil, nil
}

func (c *Client) fetchHTML(ctx context.Context, path string, params url.Values) (string, error) {
	u := siteURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept-Language", "sv-SE,sv;q=0.9")
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "wayke", httputil.DefaultRetryPolicy())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("wayke scrape: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func searchPath(q *string) string {
	if q == nil {
		return "/sok"
	}
	parts := strings.Fields(strings.TrimSpace(*q))
	if len(parts) == 0 {
		return "/sok"
	}
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segments = append(segments, slugSegment(part))
	}
	return "/sok/" + strings.Join(segments, "/")
}

func slugSegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.ReplaceAll(s, " ", "-")
}

func extractDocuments(html string) []map[string]any {
	locs := vehicleDocRE.FindAllStringIndex(html, -1)
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(locs))
	for _, loc := range locs {
		obj, ok := decodeJSONObject(html, loc[0])
		if !ok {
			continue
		}
		id := fmt.Sprint(obj["_id"])
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, obj)
	}
	return out
}

func decodeJSONObject(s string, start int) (map[string]any, bool) {
	if start < 0 || start >= len(s) || s[start] != '{' {
		return nil, false
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			switch ch {
			case '\\':
				esc = true
			case '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var obj map[string]any
				if json.Unmarshal([]byte(s[start:i+1]), &obj) != nil {
					return nil, false
				}
				return obj, true
			}
		}
	}
	return nil, false
}

func parseScrapedVehicle(item map[string]any) schema.CarListing {
	id := fmt.Sprint(item["_id"])
	listing := schema.CarListing{
		Source:             "wayke",
		ID:                 id,
		Title:              str(item["title"]),
		Make:               strPtr(item, "manufacturer"),
		Model:              strPtr(item, "modelSeries"),
		Year:               intFrom(item["modelYear"]),
		MileageKM:          mileageKMFromScrape(item),
		PriceSEK:           intFrom(item["price"]),
		Fuel:               strPtr(item, "fuelType"),
		Transmission:       strPtr(item, "gearboxType"),
		Location:           cityName(item),
		DealerName:         branchName(item),
		URL:                schema.StrPtr(listingURL(id)),
		ImageURL:           featuredImageURL(item),
		PublishedAt:        strPtr(item, "itemPublished"),
		RegistrationNumber: strPtr(item, "registrationNumber"),
		Raw:                item,
	}
	soh.Apply(&listing, "wayke_search", listing.Title, str(item["shortDescription"]))
	return listing
}

func parseJSONLDCar(data map[string]any, id string) schema.CarListing {
	brand, _ := data["brand"].(map[string]any)
	offers, _ := data["offers"].(map[string]any)
	seller, _ := offers["seller"].(map[string]any)
	engine, _ := data["vehicleEngine"].(map[string]any)
	listing := schema.CarListing{
		Source:             "wayke",
		ID:                 id,
		Title:              str(data["name"]),
		Make:               strPtr(brand, "name"),
		Model:              strPtr(data, "model"),
		Year:               yearFromJSONLD(data["vehicleModelDate"]),
		MileageKM:          mileageKMFromJSONLD(data["mileageFromOdometer"]),
		PriceSEK:           intFrom(offers["price"]),
		Fuel:               strPtr(engine, "fuelType"),
		Transmission:       strPtr(data, "vehicleTransmission"),
		DealerName:         strPtr(seller, "name"),
		URL:                schema.StrPtr(listingURL(id)),
		ImageURL:           firstImageURL(data["image"]),
		PublishedAt:        strPtr(offers, "validFrom"),
		RegistrationNumber: registrationFromJSONLD(data["identifier"]),
		Raw:                map[string]any{"jsonld": data},
	}
	return listing
}

func detailFieldsFromJSONLD(data map[string]any) []string {
	fields := []string{
		str(data["name"]),
		str(data["vehicleConfiguration"]),
		str(data["color"]),
		str(data["bodyType"]),
	}
	if engine, ok := data["vehicleEngine"].(map[string]any); ok {
		fields = append(fields, str(engine["fuelType"]))
	}
	return fields
}

func listingURL(id string) string {
	return siteURL + "/objekt/" + strings.Trim(strings.TrimSpace(id), "/")
}

func branchName(item map[string]any) *string {
	branches, _ := item["branches"].([]any)
	if len(branches) == 0 {
		return nil
	}
	branch, _ := branches[0].(map[string]any)
	return strPtr(branch, "name")
}

func cityName(item map[string]any) *string {
	position, _ := item["position"].(map[string]any)
	return strPtr(position, "city")
}

func mileageKMFromScrape(item map[string]any) *int {
	v := intFrom(item["mileage"])
	if v == nil {
		return nil
	}
	if odo, ok := item["odometerReading"].(map[string]any); ok {
		if strings.EqualFold(str(odo["unit"]), "ScandinavianMile") {
			km := *v * 10
			return &km
		}
	}
	return v
}

func mileageKMFromJSONLD(v any) *int {
	reading, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	raw := strings.TrimSpace(str(reading["value"]))
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &n
}

func yearFromJSONLD(v any) *int {
	raw := strings.TrimSpace(str(v))
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &n
}

func registrationFromJSONLD(v any) *string {
	identifier, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	if fmt.Sprint(identifier["propertyID"]) != "registrationNumber" {
		return nil
	}
	return strPtr(identifier, "value")
}

func firstImageURL(v any) *string {
	switch typed := v.(type) {
	case string:
		return schema.StrPtr(typed)
	case []any:
		if len(typed) == 0 {
			return nil
		}
		if s, ok := typed[0].(string); ok {
			return schema.StrPtr(s)
		}
	}
	return nil
}

func featuredImageURL(item map[string]any) *string {
	featured, _ := item["featuredImage"].(map[string]any)
	files, _ := featured["files"].([]any)
	if len(files) == 0 {
		return nil
	}
	file, _ := files[0].(map[string]any)
	formats, _ := file["formats"].([]any)
	for _, format := range formats {
		f, ok := format.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(f["format"]) == "770x514" {
			return strPtr(f, "url")
		}
	}
	if len(formats) > 0 {
		if f, ok := formats[0].(map[string]any); ok {
			return strPtr(f, "url")
		}
	}
	return nil
}
