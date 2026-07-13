package wayke

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

const restURL = "https://api.wayke.se/vehicles"

// Client queries Wayke via REST (with dealer API key) or public website scrape.
type Client struct {
	httpClient *http.Client
	owns       bool
	apiKey     string
}

func NewClient(c *http.Client, apiKey string) *Client {
	if c == nil {
		c = httputil.NewClient()
	}
	if apiKey == "" {
		apiKey = os.Getenv("WAYKE_API_KEY")
	}
	return &Client{httpClient: c, owns: c.Transport == nil, apiKey: apiKey}
}

func parseVehicle(item map[string]any) schema.CarListing {
	manufacturer, _ := item["manufacturer"].(map[string]any)
	series, _ := item["modelSeries"].(map[string]any)
	org, _ := item["organization"].(map[string]any)
	image, _ := item["image"].(map[string]any)
	var mileageKM, priceSEK, year *int
	if v := intFrom(item["mileage"]); v != nil {
		mileageKM = v
	}
	if v := intFrom(item["price"]); v != nil {
		priceSEK = v
	}
	if v := intFrom(item["modelYear"]); v != nil {
		year = v
	}
	listing := schema.CarListing{
		Source:             "wayke",
		ID:                 fmt.Sprint(item["id"]),
		Title:              str(item["title"]),
		Make:               strPtr(manufacturer, "name"),
		Model:              strPtr(series, "name"),
		Year:               year,
		MileageKM:          mileageKM,
		PriceSEK:           priceSEK,
		Fuel:               strPtr(item, "fuelType"),
		Transmission:       strPtr(item, "gearbox"),
		Location:           strPtr(item, "city"),
		DealerName:         strPtr(org, "name"),
		URL:                strPtr(item, "url"),
		ImageURL:           strPtr(image, "url"),
		PublishedAt:        strPtr(item, "published"),
		RegistrationNumber: strPtr(item, "registrationNumber"),
		Raw:                item,
	}
	soh.Apply(&listing, "wayke_search", listing.Title, str(item["shortDescription"]))
	return listing
}

func detailFields(item map[string]any) []string {
	fields := []string{str(item["description"]), str(item["shortDescription"])}
	data, _ := item["data"].(map[string]any)
	for _, key := range []string{"properties", "options"} {
		coll := data[key]
		switch typed := coll.(type) {
		case []any:
			for _, entry := range typed {
				if m, ok := entry.(map[string]any); ok {
					fields = append(fields, str(m["name"]), str(m["label"]), str(m["value"]))
				} else {
					fields = append(fields, fmt.Sprint(entry))
				}
			}
		case map[string]any:
			for k, v := range typed {
				fields = append(fields, fmt.Sprintf("%s: %v", k, v))
			}
		}
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			out = append(out, f)
		}
	}
	return out
}

func enrichSOH(listing *schema.CarListing, item map[string]any) *schema.CarListing {
	soh.Apply(listing, "wayke_detail", detailFields(item)...)
	if listing.Raw == nil {
		listing.Raw = map[string]any{}
	}
	listing.Raw["detail"] = item
	return listing
}

func (c *Client) Search(ctx context.Context, q *string, rows, page int) ([]schema.CarListing, error) {
	if c.apiKey != "" {
		rest, err := c.searchREST(ctx, q, rows, page)
		if err == nil && rest != nil {
			return rest, nil
		}
	}
	return c.searchScrape(ctx, q, rows, page)
}

func (c *Client) searchREST(ctx context.Context, q *string, rows, page int) ([]schema.CarListing, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	query := req.URL.Query()
	query.Set("take", fmt.Sprint(rows))
	query.Set("skip", fmt.Sprint((page-1)*rows))
	if q != nil && *q != "" {
		query.Set("search", *q)
	}
	req.URL.RawQuery = query.Encode()
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "wayke", httputil.DefaultRetryPolicy())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("wayke rest: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var items []any
	switch typed := payload.(type) {
	case []any:
		items = typed
	case map[string]any:
		if v, ok := typed["vehicles"].([]any); ok {
			items = v
		} else if v, ok := typed["data"].([]any); ok {
			items = v
		}
	}
	out := make([]schema.CarListing, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, parseVehicle(m))
		}
	}
	return out, nil
}

func (c *Client) GetVehicle(ctx context.Context, vehicleID string) (*schema.CarListing, error) {
	if c.apiKey != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, restURL+"/"+vehicleID, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
			resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "wayke", httputil.DefaultRetryPolicy())
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					body, _ := io.ReadAll(resp.Body)
					var item map[string]any
					if json.Unmarshal(body, &item) == nil {
						listing := parseVehicle(item)
						return enrichSOH(&listing, item), nil
					}
				}
			}
		}
	}
	return c.getVehicleScrape(ctx, vehicleID)
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func strPtr(m map[string]any, key string) *string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" {
		return nil
	}
	return &s
}

func intFrom(v any) *int {
	if v == nil {
		return nil
	}
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
