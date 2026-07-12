package blocket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

const (
	searchURL = "https://www.blocket.se/mobility/search/api/search/SEARCH_ID_CAR_USED"
	detailURL = "https://blocket-api.se/v1/ad/car"
)

var (
	fuelMap = map[string]string{
		"el": "4", "electric": "4", "bensin": "1", "diesel": "2",
	}
	transmissionMap = map[string]string{
		"manuell": "1", "manual": "1", "automat": "2", "automatic": "2", "automatisk": "2",
	}
)

// SearchParams holds Blocket search query parameters.
type SearchParams struct {
	Q            *string
	Make         *string
	Model        *string
	PriceFrom    *int
	PriceTo      *int
	YearFrom     *int
	YearTo       *int
	MileageToKM  *int
	Fuel         any
	Transmission any
	Sort         *string
	Rows         int
	Page         int
}

func normalizeMake(make string) string {
	if make == "" {
		return ""
	}
	key := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(make), " ", "_"), "-", "_"))
	if key == "MERCEDES" || key == "MERCEDES_BENZ" {
		return "MERCEDES_BENZ"
	}
	return key
}

// BuildParams converts high-level filters to Blocket API params.
func BuildParams(p SearchParams) map[string]string {
	params := map[string]string{}
	if p.Q != nil && *p.Q != "" {
		params["q"] = *p.Q
	}
	mk := ""
	if p.Make != nil {
		mk = normalizeMake(*p.Make)
	}
	if mk != "" && p.Model != nil && *p.Model != "" {
		params["variant"] = mk + "/" + strings.TrimSpace(*p.Model)
	} else if mk != "" {
		params["make"] = mk
	} else if p.Model != nil && *p.Model != "" {
		params["q"] = strings.TrimSpace(*p.Model)
	}
	if p.PriceFrom != nil {
		params["price_from"] = strconv.Itoa(*p.PriceFrom)
	}
	if p.PriceTo != nil {
		params["price_to"] = strconv.Itoa(*p.PriceTo)
	}
	if p.YearFrom != nil {
		params["year_from"] = strconv.Itoa(*p.YearFrom)
	}
	if p.YearTo != nil {
		params["year_to"] = strconv.Itoa(*p.YearTo)
	}
	if p.MileageToKM != nil {
		params["mileage_to"] = strconv.Itoa(max(1, *p.MileageToKM/10))
	}
	if p.Fuel != nil {
		switch v := p.Fuel.(type) {
		case int:
			params["fuel"] = strconv.Itoa(v)
		case string:
			if _, err := strconv.Atoi(v); err == nil {
				params["fuel"] = v
			} else if mapped, ok := fuelMap[strings.ToLower(v)]; ok {
				params["fuel"] = mapped
			} else {
				params["fuel"] = v
			}
		}
	}
	if p.Transmission != nil {
		switch v := p.Transmission.(type) {
		case int:
			params["transmission"] = strconv.Itoa(v)
		case string:
			if _, err := strconv.Atoi(v); err == nil {
				params["transmission"] = v
			} else if mapped, ok := transmissionMap[strings.ToLower(v)]; ok {
				params["transmission"] = mapped
			} else {
				params["transmission"] = v
			}
		}
	}
	if p.Sort != nil && *p.Sort != "" {
		params["sort"] = *p.Sort
	}
	rows := p.Rows
	if rows == 0 {
		rows = 20
	}
	params["rows"] = strconv.Itoa(rows)
	page := p.Page
	if page == 0 {
		page = 1
	}
	start := (page - 1) * rows
	if start > 0 {
		params["start"] = strconv.Itoa(start)
		params["page"] = strconv.Itoa(page)
	}
	return params
}

func pickStr(m map[string]any, key string) *string {
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

func pickInt(m map[string]any, key string) *int {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case int:
		return &n
	case json.Number:
		i, _ := n.Int64()
		v := int(i)
		return &v
	default:
		i, err := strconv.Atoi(fmt.Sprint(v))
		if err != nil {
			return nil
		}
		return &i
	}
}

func strVal(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

func coalesceStr(values ...*string) *string {
	for _, v := range values {
		if v != nil && *v != "" {
			return v
		}
	}
	return nil
}

func titleFrom(p map[string]any) string {
	if h := pickStr(p, "heading"); h != nil {
		return *h
	}
	if f := pickStr(p, "facade_title"); f != nil {
		return *f
	}
	return ""
}

// ParseAd converts a Blocket search document to CarListing.
func ParseAd(p map[string]any) schema.CarListing {
	mileage := pickInt(p, "mileage")
	var mileageKM *int
	if mileage != nil {
		v := *mileage * 10
		mileageKM = &v
	}
	price, _ := p["price"].(map[string]any)
	var priceSEK *int
	if price != nil {
		priceSEK = pickInt(price, "amount")
	}
	image, _ := p["image"].(map[string]any)
	var imageURL *string
	if image != nil {
		imageURL = pickStr(image, "url")
	}
	var urlList []any
	if raw, ok := p["image_urls"].([]any); ok {
		urlList = raw
	}
	var published *string
	if ts := pickInt(p, "timestamp"); ts != nil {
		t := time.Unix(int64(*ts)/1000, 0).UTC().Format(time.RFC3339)
		published = &t
	}
	listingID := ""
	if id := pickInt(p, "ad_id"); id != nil {
		listingID = strconv.Itoa(*id)
	} else if id := pickStr(p, "id"); id != nil {
		listingID = *id
	}
	if imageURL == nil && len(urlList) > 0 {
		if s, ok := urlList[0].(string); ok {
			imageURL = &s
		}
	}
	listing := schema.CarListing{
		Source:             "blocket",
		ID:                 listingID,
		Title:              titleFrom(p),
		Make:               pickStr(p, "make"),
		Model:              pickStr(p, "model"),
		Year:               pickInt(p, "year"),
		MileageKM:          mileageKM,
		PriceSEK:           priceSEK,
		Fuel:               pickStr(p, "fuel"),
		Transmission:       pickStr(p, "transmission"),
		Location:           pickStr(p, "location"),
		DealerName:         coalesceStr(pickStr(p, "organisation_name"), pickStr(p, "dealer_segment")),
		URL:                pickStr(p, "canonical_url"),
		ImageURL:           imageURL,
		PublishedAt:        published,
		RegistrationNumber: pickStr(p, "regno"),
		Raw:                p,
	}
	fields := []string{strVal(p, "model_specification"), listing.Title}
	for _, key := range []string{"extras", "labels"} {
		if arr, ok := p[key].([]any); ok {
			for _, item := range arr {
				fields = append(fields, fmt.Sprint(item))
			}
		}
	}
	soh.Apply(&listing, "blocket_search", fields...)
	return listing
}

// Client queries Blocket mobility APIs.
type Client struct {
	httpClient *http.Client
	owns       bool
}

func NewClient(c *http.Client) *Client {
	if c == nil {
		return &Client{httpClient: httputil.NewClient(), owns: true}
	}
	return &Client{httpClient: c, owns: false}
}

func (c *Client) Search(ctx context.Context, p SearchParams) ([]schema.CarListing, error) {
	params := BuildParams(p)
	u, _ := url.Parse(searchURL)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("blocket search: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	docs, _ := data["docs"].([]any)
	out := make([]schema.CarListing, 0, len(docs))
	for _, doc := range docs {
		if m, ok := doc.(map[string]any); ok {
			out = append(out, ParseAd(m))
		}
	}
	return out, nil
}

func (c *Client) GetListing(ctx context.Context, listingID string) (*schema.CarListing, error) {
	q := listingID
	results, err := c.Search(ctx, SearchParams{Q: &q, Rows: 60, Page: 1})
	if err != nil {
		return nil, err
	}
	for i := range results {
		if results[i].ID == listingID {
			return c.enrichListing(ctx, &results[i])
		}
	}
	return nil, nil
}

func (c *Client) enrichListing(ctx context.Context, listing *schema.CarListing) (*schema.CarListing, error) {
	if listing.ID == "" {
		return listing, nil
	}
	u, _ := url.Parse(detailURL)
	q := u.Query()
	q.Set("id", listing.ID)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return listing, nil
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return listing, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return listing, nil
	}
	var detail map[string]any
	if err := json.Unmarshal(body, &detail); err != nil {
		return listing, nil
	}
	fields := []string{strVal(detail, "subtitle")}
	if equipment, ok := detail["equipment"].([]any); ok {
		for _, item := range equipment {
			switch v := item.(type) {
			case map[string]any:
				fields = append(fields, strVal(v, "name"), strVal(v, "label"))
			default:
				fields = append(fields, fmt.Sprint(item))
			}
		}
	}
	soh.Apply(listing, "blocket_detail", fields...)
	if listing.Raw == nil {
		listing.Raw = map[string]any{}
	}
	listing.Raw["detail"] = detail
	return listing, nil
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}
