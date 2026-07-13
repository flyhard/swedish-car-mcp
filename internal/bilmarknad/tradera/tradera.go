package tradera

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

const (
	apiBase           = "https://api.tradera.com/v3"
	searchURL         = apiBase + "/searchservice.asmx/Search"
	getItemURL        = apiBase + "/publicservice.asmx/GetItem"
	defaultCategoryID = 10
	cacheTTL          = 30 * time.Minute
)

// UnavailableError indicates Tradera API failure or missing credentials.
type UnavailableError struct{ Msg string }

func (e UnavailableError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "tradera unavailable"
}

type timedCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	at    time.Time
	value any
}

func newTimedCache() *timedCache {
	return &timedCache{entries: map[string]cacheEntry{}}
}

func (c *timedCache) get(key string, ttl time.Duration) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.at) > ttl {
		delete(c.entries, key)
		return nil, false
	}
	return entry.value, true
}

func (c *timedCache) set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{at: time.Now(), value: value}
}

// Client queries Tradera REST v3 XML APIs.
type Client struct {
	httpClient    *http.Client
	owns          bool
	appID         string
	appKey        string
	carCategoryID int
	cache         *timedCache
}

func NewClient(c *http.Client, appID, appKey string, carCategoryID int) *Client {
	if c == nil {
		c = httputil.NewClient()
	}
	if appID == "" {
		appID = os.Getenv("TRADERA_APP_ID")
		if appID == "" {
			appID = "5572"
		}
	}
	if appKey == "" {
		appKey = os.Getenv("TRADERA_APP_KEY")
		if appKey == "" {
			appKey = "81974dd3-404d-456e-b050-b030ba646d6a"
		}
	}
	if carCategoryID == 0 {
		if env := os.Getenv("TRADERA_CAR_CATEGORY_ID"); env != "" {
			if v, err := strconv.Atoi(env); err == nil {
				carCategoryID = v
			}
		}
	}
	if carCategoryID == 0 {
		carCategoryID = defaultCategoryID
	}
	return &Client{
		httpClient:    c,
		owns:          true,
		appID:         appID,
		appKey:        appKey,
		carCategoryID: carCategoryID,
		cache:         newTimedCache(),
	}
}

func localTag(tag string) string {
	if i := strings.LastIndex(tag, "}"); i >= 0 {
		return tag[i+1:]
	}
	return tag
}

func assignKey(m map[string]any, key string, val any) {
	if existing, ok := m[key]; ok {
		switch e := existing.(type) {
		case []any:
			m[key] = append(e, val)
		default:
			m[key] = []any{e, val}
		}
	} else {
		m[key] = val
	}
}

// ParseXMLResponse parses Tradera XML into a nested map keyed by root tag.
func ParseXMLResponse(text string) (map[string]any, error) {
	decoder := xml.NewDecoder(strings.NewReader(text))
	var root map[string]any
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			val, err := parseElement(&start, decoder)
			if err != nil {
				return nil, err
			}
			root = map[string]any{localTag(start.Name.Local): val}
			break
		}
	}
	return root, nil
}

func parseElement(start *xml.StartElement, decoder *xml.Decoder) (any, error) {
	var children []any
	var textParts []string
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseElement(&t, decoder)
			if err != nil {
				return nil, err
			}
			children = append(children, map[string]any{localTag(t.Name.Local): child})
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if len(children) == 0 {
					if len(textParts) > 0 {
						return strings.Join(textParts, ""), nil
					}
					return nil, nil
				}
				result := map[string]any{}
				for _, child := range children {
					for k, v := range child.(map[string]any) {
						assignKey(result, k, v)
					}
				}
				return result, nil
			}
		case xml.CharData:
			s := strings.TrimSpace(string(t))
			if s != "" {
				textParts = append(textParts, s)
			}
		}
	}
}

func asList(v any) []map[string]any {
	if v == nil {
		return nil
	}
	switch typed := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		if _, ok := typed["Id"]; ok {
			return []map[string]any{typed}
		}
		if item, ok := typed["Item"]; ok {
			return asList(item)
		}
		return []map[string]any{typed}
	default:
		return nil
	}
}

func dig(data map[string]any, keys ...string) any {
	current := any(data)
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		var matched any
		for k, v := range m {
			if localTag(k) == key || k == key {
				matched = v
				break
			}
		}
		current = matched
	}
	return current
}

func textVal(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func toInt(v any) *int {
	if v == nil {
		return nil
	}
	if p, ok := v.(*int); ok {
		return p
	}
	switch n := v.(type) {
	case int:
		return &n
	case float64:
		i := int(n)
		return &i
	default:
		f, err := strconv.ParseFloat(textVal(v), 64)
		if err != nil {
			return nil
		}
		i := int(f)
		return &i
	}
}

func parseAttributes(attrValues any) map[string]string {
	attrs := map[string]string{}
	m, ok := attrValues.(map[string]any)
	if !ok {
		return attrs
	}
	term := dig(m, "TermAttributeValues", "TermAttributeValue")
	for _, entry := range asList(term) {
		name := textVal(entry["Name"])
		values := entry["Values"]
		var value string
		switch v := values.(type) {
		case map[string]any:
			value = textVal(v["string"])
		case []any:
			if len(v) > 0 {
				value = textVal(v[0])
			}
		default:
			value = textVal(v)
		}
		if name == "" || value == "" {
			continue
		}
		key := strings.ToLower(name)
		switch key {
		case "condition", "skick":
			attrs["condition"] = value
		case "mobile_brand", "märke", "brand", "make":
			attrs["brand"] = value
		case "mobile_model", "modell", "model":
			attrs["model"] = value
		}
	}
	return attrs
}

func parseImageURLs(imageLinks any) []string {
	var urls []string
	for _, entry := range asList(imageLinks) {
		if u := textVal(entry["Url"]); u != "" {
			urls = append(urls, u)
		} else if u := textVal(entry["url"]); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func parseItem(item map[string]any, detailed bool) map[string]any {
	attrs := parseAttributes(item["AttributeValues"])
	seller, _ := item["Seller"].(map[string]any)
	imageURLs := parseImageURLs(item["ImageLinks"])
	itemID := toInt(item["Id"])
	if itemID == nil {
		itemID = toInt(item["ItemId"])
	}
	id := 0
	if itemID != nil {
		id = *itemID
	}
	parsed := map[string]any{
		"item_id":           id,
		"short_description": textVal(item["ShortDescription"]),
		"long_description":  textVal(item["LongDescription"]),
		"category_id":       toInt(item["CategoryId"]),
		"category_name":     textVal(item["CategoryName"]),
		"seller_id":         toInt(item["SellerId"]),
		"seller_alias":      textVal(item["SellerAlias"]),
		"seller_city":       coalesceText(textVal(item["SellerCity"]), textVal(seller["City"])),
		"start_price":       toInt(item["StartPrice"]),
		"buy_it_now_price":  toInt(item["BuyItNowPrice"]),
		"current_bid":       coalesceInt(toInt(item["MaxBid"]), toInt(item["CurrentBid"])),
		"bid_count":         toInt(item["TotalBids"]),
		"start_date":        textVal(item["StartDate"]),
		"end_date":          textVal(item["EndDate"]),
		"thumbnail_url":     textVal(item["ThumbnailLink"]),
		"image_urls":        imageURLs,
		"item_type":         coalesceText(textVal(item["ItemType"]), "Auction"),
		"item_url":          textVal(item["ItemUrl"]),
		"attributes":        attrs,
	}
	if parsed["short_description"] == "" {
		parsed["short_description"] = textVal(item["Title"])
	}
	if detailed {
		parsed["seller_rating"] = toInt(item["SellerDsrAverage"])
		if parsed["seller_rating"] == nil {
			parsed["seller_rating"] = toInt(seller["TotalRating"])
		}
	}
	return parsed
}

func coalesceText(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func coalesceInt(values ...*int) *int {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func priceSEK(item map[string]any) *int {
	for _, key := range []string{"buy_it_now_price", "current_bid", "start_price"} {
		if v := toInt(item[key]); v != nil && *v > 0 {
			return v
		}
	}
	return nil
}

// ParseTraderaItem converts parsed Tradera item data to CarListing.
func ParseTraderaItem(raw map[string]any, detailed bool) schema.CarListing {
	item := parseItem(raw, detailed)
	attrs, _ := item["attributes"].(map[string]string)
	imageURLs, _ := item["image_urls"].([]string)
	thumbnail := textVal(item["thumbnail_url"])
	var imageURL *string
	if thumbnail != "" {
		imageURL = &thumbnail
	} else if len(imageURLs) > 0 {
		imageURL = &imageURLs[0]
	}
	itemID := fmt.Sprint(item["item_id"])
	itemURL := textVal(item["item_url"])
	if itemURL == "" {
		itemURL = "https://www.tradera.com/item/" + itemID
	}
	listing := schema.CarListing{
		Source:      "tradera",
		ID:          itemID,
		Title:       textVal(item["short_description"]),
		Make:        strPtr(attrs["brand"]),
		Model:       strPtr(attrs["model"]),
		PriceSEK:    priceSEK(item),
		Location:    strPtr(textVal(item["seller_city"])),
		DealerName:  strPtr(textVal(item["seller_alias"])),
		URL:         &itemURL,
		ImageURL:    imageURL,
		PublishedAt: strPtr(textVal(item["start_date"])),
		Raw:         item,
	}
	source := "tradera_search"
	if detailed {
		source = "tradera_detail"
	}
	soh.Apply(&listing, source, textVal(item["short_description"]), textVal(item["long_description"]))
	return listing
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func extractSearchItems(parsed map[string]any) []map[string]any {
	items := dig(parsed, "SearchResult", "Items")
	if items == nil {
		items = dig(parsed, "Items")
	}
	return asList(items)
}

func extractItem(parsed map[string]any) map[string]any {
	if item, ok := dig(parsed, "Item").(map[string]any); ok {
		return item
	}
	for _, v := range parsed {
		if m, ok := v.(map[string]any); ok {
			if _, ok := m["Id"]; ok {
				return m
			}
			if _, ok := m["ItemId"]; ok {
				return m
			}
		}
	}
	return nil
}

func (c *Client) authParams() (map[string]string, error) {
	if c.appID == "" || c.appKey == "" {
		return nil, UnavailableError{Msg: "TRADERA_APP_ID and TRADERA_APP_KEY are required"}
	}
	return map[string]string{"appId": c.appID, "appKey": c.appKey}, nil
}

func (c *Client) getXML(ctx context.Context, url string, params map[string]string) (map[string]any, error) {
	auth, err := c.authParams()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range auth {
		q.Set(k, v)
	}
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "tradera", httputil.DefaultRetryPolicy())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, UnavailableError{Msg: fmt.Sprintf("Tradera API returned %d", resp.StatusCode)}
	}
	return ParseXMLResponse(string(body))
}

func (c *Client) Search(ctx context.Context, q *string, rows, page int, orderBy string) ([]schema.CarListing, error) {
	query := "bil"
	if q != nil && strings.TrimSpace(*q) != "" {
		query = strings.TrimSpace(*q)
	}
	if orderBy == "" {
		orderBy = "Relevance"
	}
	cacheKey := fmt.Sprintf("search:%s:%d:%s:%d:%d", query, c.carCategoryID, orderBy, rows, page)
	if cached, ok := c.cache.get(cacheKey, cacheTTL); ok {
		return cached.([]schema.CarListing), nil
	}
	parsed, err := c.getXML(ctx, searchURL, map[string]string{
		"query":      query,
		"categoryId": strconv.Itoa(c.carCategoryID),
		"orderBy":    orderBy,
		"pageNumber": strconv.Itoa(page),
	})
	if err != nil {
		return nil, err
	}
	items := extractSearchItems(parsed)
	out := make([]schema.CarListing, 0, len(items))
	for _, item := range items {
		out = append(out, ParseTraderaItem(item, false))
	}
	if len(out) > rows {
		out = out[:rows]
	}
	c.cache.set(cacheKey, out)
	return out, nil
}

func (c *Client) GetListing(ctx context.Context, listingID string) (*schema.CarListing, error) {
	cacheKey := "item:" + listingID
	if cached, ok := c.cache.get(cacheKey, cacheTTL); ok {
		l := cached.(schema.CarListing)
		return &l, nil
	}
	parsed, err := c.getXML(ctx, getItemURL, map[string]string{"itemId": listingID})
	if err != nil {
		return nil, err
	}
	raw := extractItem(parsed)
	if raw == nil {
		return nil, nil
	}
	listing := ParseTraderaItem(raw, true)
	c.cache.set(cacheKey, listing)
	return &listing, nil
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}
