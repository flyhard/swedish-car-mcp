package riddermark

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

const baseURL = "https://www.riddermarkbil.se"

var nextDataRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// Client scrapes Riddermark Next.js page data.
type Client struct {
	httpClient *http.Client
	owns       bool
}

func NewClient(c *http.Client) *Client {
	if c == nil {
		c = httputil.NewRedirectClient(map[string]string{"User-Agent": httputil.UserAgent})
	}
	return &Client{httpClient: c, owns: c.Transport == nil}
}

func makeSlug(make string) string {
	return url.PathEscape(strings.ToLower(strings.TrimSpace(make)))
}

func listingID(make, licensePlate string) string {
	return makeSlug(make) + "/" + strings.ToLower(strings.TrimSpace(licensePlate))
}

func listingURL(make, licensePlate string) string {
	return baseURL + "/kopa-bil/" + listingID(make, licensePlate) + "/"
}

func coverImage(item map[string]any) *string {
	if cover, ok := item["coverImage"].(map[string]any); ok {
		if u, ok := cover["url"].(string); ok && u != "" {
			return &u
		}
	}
	if images, ok := item["images"].([]any); ok && len(images) > 0 {
		if img, ok := images[0].(map[string]any); ok {
			if u, ok := img["url"].(string); ok && u != "" {
				return &u
			}
		}
	}
	return nil
}

func locationName(item map[string]any) *string {
	for _, key := range []string{"location", "physicalLocation"} {
		if loc, ok := item[key].(map[string]any); ok {
			if name, ok := loc["name"].(string); ok && name != "" {
				return &name
			}
		}
	}
	return nil
}

func parseCar(item map[string]any, detail bool) schema.CarListing {
	make := fmt.Sprint(item["make"])
	licensePlate := fmt.Sprint(item["licenseplate"])
	dealer := "Riddermark Bil"
	listing := schema.CarListing{
		Source:             "riddermark",
		ID:                 listingID(make, licensePlate),
		Title:              coalesceTitle(item),
		Make:               strPtr(make),
		Model:              strPtr(fmt.Sprint(item["model"])),
		Year:               intPtr(item["modelYear"]),
		MileageKM:          intPtr(item["mileage"]),
		PriceSEK:           intPtr(item["price"]),
		Fuel:               strPtr(fmt.Sprint(item["fuelType"])),
		Transmission:       strPtr(fmt.Sprint(item["gearboxType"])),
		Location:           locationName(item),
		DealerName:         &dealer,
		URL:                schema.StrPtr(listingURL(make, licensePlate)),
		ImageURL:           coverImage(item),
		PublishedAt:        strPtr(fmt.Sprint(item["publishedAt"])),
		RegistrationNumber: strPtr(licensePlate),
		Raw:                item,
	}
	source := "riddermark_search"
	if detail {
		source = "riddermark_detail"
	}
	soh.Apply(&listing, source,
		fmt.Sprint(item["title"]),
		fmt.Sprint(item["modelDescription"]),
		fmt.Sprint(item["fullText"]),
	)
	return listing
}

func coalesceTitle(item map[string]any) string {
	if t := fmt.Sprint(item["title"]); t != "" && t != "<nil>" {
		return t
	}
	return fmt.Sprint(item["carName"])
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

func (c *Client) fetchNextData(ctx context.Context, path string, params url.Values) (map[string]any, error) {
	u := baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("riddermark: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return extractNextData(string(body)), nil
}

func (c *Client) Search(ctx context.Context, q, makeName, model *string, priceMin, priceMax, mileageMax *int, rows, page int) ([]schema.CarListing, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprint(page))
	var parts []string
	for _, p := range []*string{q, makeName, model} {
		if p != nil && *p != "" {
			parts = append(parts, *p)
		}
	}
	if len(parts) > 0 {
		params.Set("search", strings.Join(parts, " "))
	}
	if priceMin != nil {
		params.Set("priceFrom", fmt.Sprint(*priceMin))
	}
	if priceMax != nil {
		params.Set("priceTo", fmt.Sprint(*priceMax))
	}
	if mileageMax != nil {
		params.Set("mileageTo", fmt.Sprint(*mileageMax))
	}
	data, err := c.fetchNextData(ctx, "/kopa-bil/", params)
	if err != nil {
		return nil, err
	}
	cars := digCars(data)
	out := make([]schema.CarListing, 0, len(cars))
	for _, item := range cars {
		out = append(out, parseCar(item, false))
	}
	if rows > 0 && len(out) > rows {
		out = out[:rows]
	}
	return out, nil
}

func digCars(data map[string]any) []map[string]any {
	props, _ := data["props"].(map[string]any)
	pageProps, _ := props["pageProps"].(map[string]any)
	cars, _ := pageProps["carsJson"].([]any)
	out := make([]map[string]any, 0, len(cars))
	for _, car := range cars {
		if m, ok := car.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (c *Client) GetListing(ctx context.Context, listingID string) (*schema.CarListing, error) {
	listingID = strings.Trim(strings.TrimSpace(listingID), "/")
	if listingID == "" {
		return nil, nil
	}
	if strings.Contains(listingID, "/") {
		parts := strings.SplitN(listingID, "/", 2)
		path := fmt.Sprintf("/kopa-bil/%s/%s/", parts[0], parts[1])
		data, err := c.fetchNextData(ctx, path, nil)
		if err != nil {
			return nil, err
		}
		props, _ := data["props"].(map[string]any)
		pageProps, _ := props["pageProps"].(map[string]any)
		if advert, ok := pageProps["advertJson"].(map[string]any); ok {
			l := parseCar(advert, true)
			return &l, nil
		}
		return nil, nil
	}
	results, err := c.Search(ctx, &listingID, nil, nil, nil, nil, nil, 5, 1)
	if err != nil {
		return nil, err
	}
	target := strings.ToLower(listingID)
	for _, item := range results {
		if item.RegistrationNumber != nil && strings.ToLower(*item.RegistrationNumber) == target {
			return c.GetListing(ctx, item.ID)
		}
		parts := strings.SplitN(item.ID, "/", 2)
		if len(parts) == 2 && parts[1] == target {
			return c.GetListing(ctx, item.ID)
		}
	}
	return nil, nil
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
