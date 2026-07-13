package carla

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

const baseURL = "https://www.carla.se"

var nextDataRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

var defaultHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Accept-Language": "sv-SE,sv;q=0.9",
}

// Client scrapes Carla Next.js page data.
type Client struct {
	httpClient *http.Client
	owns       bool
}

func NewClient(c *http.Client) *Client {
	if c == nil {
		c = httputil.NewRedirectClient(defaultHeaders)
	}
	return &Client{httpClient: c, owns: true}
}

func fuelLabel(carType string) *string {
	if carType == "" {
		return nil
	}
	mapping := map[string]string{
		"BEV": "El", "PHEV": "Laddhybrid", "EV": "El",
	}
	if v, ok := mapping[strings.ToUpper(carType)]; ok {
		return &v
	}
	return &carType
}

func transmissionLabel(value string) *string {
	if value == "" {
		return nil
	}
	switch strings.ToUpper(value) {
	case "AUTOMATIC":
		v := "Automat"
		return &v
	case "MANUAL":
		v := "Manuell"
		return &v
	default:
		return &value
	}
}

func title(item map[string]any) string {
	var parts []string
	for _, key := range []string{"manufacturer", "model", "version"} {
		if v := strings.TrimSpace(fmt.Sprint(item[key])); v != "" && v != "<nil>" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

func listingURL(objectID string) string {
	return baseURL + "/bil/" + strings.Trim(strings.TrimSpace(objectID), "/")
}

func detailFields(item map[string]any) []string {
	fields := []string{title(item), fmt.Sprint(item["version"]), fmt.Sprint(item["description"]), fmt.Sprint(item["batteryTestResult"])}
	if bs := item["batteryState"]; bs != nil {
		b, _ := json.Marshal(bs)
		fields = append(fields, string(b))
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.TrimSpace(f) != "" && f != "<nil>" {
			out = append(out, f)
		}
	}
	return out
}

func parseSearchCar(item map[string]any) schema.CarListing {
	objectID := fmt.Sprint(item["objectID"])
	dealer := "Carla"
	listing := schema.CarListing{
		Source:             "carla",
		ID:                 objectID,
		Title:              title(item),
		Make:               strPtr(fmt.Sprint(item["manufacturer"])),
		Model:              strPtr(fmt.Sprint(item["model"])),
		Year:               intPtr(item["modelYear"]),
		MileageKM:          intPtr(item["mileage"]),
		PriceSEK:           intPtr(item["retailPrice"]),
		Fuel:               fuelLabel(fmt.Sprint(item["type"])),
		DealerName:         &dealer,
		URL:                schema.StrPtr(listingURL(objectID)),
		ImageURL:           strPtr(fmt.Sprint(item["mainPhoto"])),
		PublishedAt:        strPtr(fmt.Sprint(item["createdAt"])),
		RegistrationNumber: strPtr(fmt.Sprint(item["registrationNumber"])),
		Raw:                item,
	}
	soh.Apply(&listing, "carla_search", listing.Title)
	return listing
}

func parseDetailCar(item map[string]any, objectID string) schema.CarListing {
	price, _ := item["price"].(map[string]any)
	dealer := "Carla"
	var imageURL *string
	if photos, ok := item["outsidePhotos"].([]any); ok && len(photos) > 0 {
		if photo, ok := photos[0].(map[string]any); ok {
			imageURL = strPtr(fmt.Sprint(photo["url"]))
		}
	}
	listing := schema.CarListing{
		Source:             "carla",
		ID:                 objectID,
		Title:              title(item),
		Make:               strPtr(fmt.Sprint(item["manufacturer"])),
		Model:              strPtr(fmt.Sprint(item["model"])),
		Year:               intPtr(item["modelYear"]),
		MileageKM:          intPtr(item["mileage"]),
		PriceSEK:           intPtr(price["retailPrice"]),
		Fuel:               fuelLabel(fmt.Sprint(item["type"])),
		Transmission:       transmissionLabel(fmt.Sprint(item["transmission"])),
		DealerName:         &dealer,
		URL:                schema.StrPtr(listingURL(objectID)),
		ImageURL:           imageURL,
		RegistrationNumber: strPtr(fmt.Sprint(item["registrationNumber"])),
		Raw:                item,
	}
	soh.Apply(&listing, "carla_detail", detailFields(item)...)
	return listing
}

func extractNextData(html string) map[string]any {
	m := nextDataRE.FindStringSubmatch(html)
	if len(m) < 2 {
		return map[string]any{}
	}
	var data map[string]any
	_ = json.Unmarshal([]byte(m[1]), &data)
	return data
}

func mapFuelType(fuel string) *string {
	if fuel == "" {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(fuel))
	switch key {
	case "el", "ev", "electric", "elektrisk", "bev":
		v := "BEV"
		return &v
	case "phev", "laddhybrid", "plugin", "plug-in":
		v := "PHEV"
		return &v
	default:
		return nil
	}
}

// MapFuelType exports fuel mapping for tests.
func MapFuelType(fuel string) *string { return mapFuelType(fuel) }

func (c *Client) fetchNextData(ctx context.Context, path string, params url.Values) (map[string]any, error) {
	u := baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "carla", httputil.DefaultRetryPolicy())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("carla: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return extractNextData(string(body)), nil
}

func (c *Client) Search(ctx context.Context, q, makeName, model, fuel *string, rows, page int) ([]schema.CarListing, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprint(page))
	if q != nil && *q != "" {
		params.Set("search", *q)
	} else if makeName != nil && model != nil && *makeName != "" && *model != "" {
		params.Set("manufacturerAndModel", *makeName+" "+*model)
	} else if makeName != nil && *makeName != "" {
		params.Set("manufacturer", *makeName)
	}
	if fuel != nil {
		if ft := mapFuelType(*fuel); ft != nil {
			params.Set("type", *ft)
		}
	}
	data, err := c.fetchNextData(ctx, "/kopa-bil", params)
	if err != nil {
		return nil, err
	}
	props, _ := data["props"].(map[string]any)
	pageProps, _ := props["pageProps"].(map[string]any)
	initial, _ := pageProps["initialResult"].(map[string]any)
	cars, _ := initial["cars"].([]any)
	out := make([]schema.CarListing, 0, len(cars))
	for _, car := range cars {
		if m, ok := car.(map[string]any); ok {
			out = append(out, parseSearchCar(m))
		}
	}
	if rows > 0 && len(out) > rows {
		out = out[:rows]
	}
	return out, nil
}

func (c *Client) GetListing(ctx context.Context, listingID string) (*schema.CarListing, error) {
	slug := strings.Trim(strings.TrimSpace(listingID), "/")
	if slug == "" {
		return nil, nil
	}
	if strings.HasPrefix(slug, "bil/") {
		slug = strings.SplitN(slug, "/", 2)[1]
	}
	if plate := schema.NormalizeRegistrationNumber(slug); plate != "" && !strings.Contains(slug, "-") && len(plate) <= 7 {
		results, err := c.Search(ctx, &plate, nil, nil, nil, 20, 1)
		if err != nil {
			return nil, err
		}
		for _, item := range results {
			if schema.RegistrationMatches(item.RegistrationNumber, plate) {
				return c.GetListing(ctx, item.ID)
			}
		}
	}
	path := "/bil/" + url.PathEscape(slug)
	data, err := c.fetchNextData(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	props, _ := data["props"].(map[string]any)
	pageProps, _ := props["pageProps"].(map[string]any)
	car, _ := pageProps["car"].(map[string]any)
	if car == nil {
		return nil, nil
	}
	pageID := slug
	if id, ok := pageProps["id"].(string); ok && id != "" {
		pageID = id
	}
	l := parseDetailCar(car, pageID)
	return &l, nil
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" || s == "<nil>" {
		return nil
	}
	return &s
}

func intPtr(v any) *int {
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case int:
		return &n
	default:
		var i int
		if _, err := fmt.Sscan(fmt.Sprint(v), &i); err != nil {
			return nil
		}
		return &i
	}
}
