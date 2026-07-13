package ayvens

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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

const (
	baseURL    = "https://usedcars.ayvens.com"
	searchPath = "/on/demandware.store/Sites-leaseplan-se-Site/sv_SE/Search-Show"
)

var (
	productClickRE    = regexp.MustCompile(`data-tracking-productclick="(\[\{.*?\}\])"`)
	productImpressRE  = regexp.MustCompile(`"event":"productImpression"`)
	itemListRE        = regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(\{.*?\})\s*</script>`)
	tileLinkRE        = regexp.MustCompile(`data-pid="([^"]+)"[\s\S]*?href="(/sv-se/[^"]+\.html)"`)
	detailContainerRE = regexp.MustCompile(`(?s)detail-container ([a-zA-Z]+)".*?<span class="value ml-0">\s*([^<]+?)\s*</span>`)
	inspectionURLRE   = regexp.MustCompile(`href="(https://bus2\.bus\.no[^"]+)"`)
	priceContentRE    = regexp.MustCompile(`class="value" content="([0-9.]+)"`)
	productNameRE     = regexp.MustCompile(`<h1 class="product-name">([^<]+)</h1>`)
	productDescRE     = regexp.MustCompile(`<p class="product-description">([^<]+)</p>`)
)

var defaultHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Accept-Language": "sv-SE,sv;q=0.9",
}

// Client searches Ayvens used-car listings (usedcars.ayvens.com).
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

func (c *Client) Search(ctx context.Context, q, makeName, model *string, priceMin, priceMax, mileageMax *int, rows, page int) ([]schema.CarListing, error) {
	if rows <= 0 {
		rows = 20
	}
	if page <= 0 {
		page = 1
	}
	params := url.Values{}
	params.Set("cgid", "cars")
	params.Set("sz", fmt.Sprint(rows))
	params.Set("start", fmt.Sprint((page-1)*rows))
	var parts []string
	for _, p := range []*string{q, makeName, model} {
		if p != nil && strings.TrimSpace(*p) != "" {
			parts = append(parts, strings.TrimSpace(*p))
		}
	}
	if len(parts) > 0 {
		params.Set("q", strings.Join(parts, " "))
	}
	body, err := c.fetch(ctx, searchPath+"?"+params.Encode())
	if err != nil {
		return nil, err
	}
	products := parseSearchProducts(body)
	out := make([]schema.CarListing, 0, len(products))
	for _, prod := range products {
		listing := parseSearchProduct(prod)
		if matchesFilters(listing, priceMin, priceMax, mileageMax) {
			out = append(out, listing)
		}
	}
	out = AttachListingPaths(out, body)
	if rows > 0 && len(out) > rows {
		out = out[:rows]
	}
	return out, nil
}

func (c *Client) GetListing(ctx context.Context, listingID string) (*schema.CarListing, error) {
	listingID = strings.Trim(strings.TrimSpace(listingID), "/")
	if listingID == "" {
		return nil, nil
	}
	path := listingPath(listingID)
	if path == "" {
		results, err := c.Search(ctx, &listingID, nil, nil, nil, nil, nil, 10, 1)
		if err != nil {
			return nil, err
		}
		target := strings.ToLower(listingID)
		for i := range results {
			if strings.EqualFold(results[i].ID, listingID) || strings.HasSuffix(strings.ToLower(results[i].ID), "/"+target) {
				path = listingPath(results[i].ID)
				if path == "" {
					if raw, ok := results[i].Raw["listing_path"].(string); ok {
						path = raw
					}
				}
				break
			}
			if results[i].RegistrationNumber != nil && schema.RegistrationMatches(results[i].RegistrationNumber, listingID) {
				path = listingPath(results[i].ID)
				if path == "" {
					if raw, ok := results[i].Raw["listing_path"].(string); ok {
						path = raw
					}
				}
				break
			}
		}
	}
	if path == "" {
		return nil, nil
	}
	body, err := c.fetch(ctx, path)
	if err != nil {
		return nil, err
	}
	listing := parseDetailHTML(body, path)
	if listing.ID == "" {
		listing.ID = listingIDFromPath(path)
	}
	if reportURL := extractInspectionURL(body); reportURL != "" {
		if listing.Raw == nil {
			listing.Raw = map[string]any{}
		}
		listing.Raw["inspection_report_url"] = reportURL
		c.enrichSOHFromInspection(ctx, &listing, reportURL)
	}
	return &listing, nil
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

func (c *Client) fetch(ctx context.Context, path string) (string, error) {
	u := path
	if strings.HasPrefix(path, "/") {
		u = baseURL + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "ayvens", httputil.DefaultRetryPolicy())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ayvens: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseSearchProducts(htmlBody string) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, block := range productClickRE.FindAllStringSubmatch(htmlBody, -1) {
		if len(block) < 2 {
			continue
		}
		raw := html.UnescapeString(block[1])
		var events []map[string]any
		if json.Unmarshal([]byte(raw), &events) != nil {
			continue
		}
		for _, ev := range events {
			ecom, _ := ev["ecommerce"].(map[string]any)
			click, _ := ecom["click"].(map[string]any)
			products, _ := click["products"].([]any)
			for _, p := range products {
				if m, ok := p.(map[string]any); ok {
					id := fmt.Sprint(m["id"])
					if id == "" {
						continue
					}
					if _, dup := seen[id]; dup {
						continue
					}
					seen[id] = struct{}{}
					out = append(out, m)
				}
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	if idx := productImpressRE.FindStringIndex(htmlBody); idx != nil {
		start := strings.LastIndex(htmlBody[:idx[0]], "data-tracking-view=\"")
		if start >= 0 {
			start += len("data-tracking-view=\"")
			end := strings.Index(htmlBody[start:], "\"")
			if end > 0 {
				raw := html.UnescapeString(htmlBody[start : start+end])
				var events []map[string]any
				if json.Unmarshal([]byte(raw), &events) == nil {
					for _, ev := range events {
						if ev["event"] != "productImpression" {
							continue
						}
						ecom, _ := ev["ecommerce"].(map[string]any)
						impressions, _ := ecom["impressions"].([]any)
						for _, p := range impressions {
							if m, ok := p.(map[string]any); ok {
								id := fmt.Sprint(m["id"])
								if id == "" {
									continue
								}
								if _, dup := seen[id]; dup {
									continue
								}
								seen[id] = struct{}{}
								out = append(out, m)
							}
						}
					}
				}
			}
		}
	}
	return out
}

func parseSearchProduct(prod map[string]any) schema.CarListing {
	id := fmt.Sprint(prod["id"])
	makeName := strings.TrimSpace(fmt.Sprint(prod["brand"]))
	modelName := strings.TrimSpace(fmt.Sprint(prod["name"]))
	title := strings.TrimSpace(strings.Join([]string{makeName, modelName}, " "))
	if variant := strings.TrimSpace(fmt.Sprint(prod["variant"])); variant != "" && variant != "<nil>" {
		if title != "" {
			title = title + " " + variant
		} else {
			title = variant
		}
	}
	var priceSEK, mileageKM, year *int
	if v, ok := prod["price"].(float64); ok {
		i := int(v)
		priceSEK = &i
	}
	if v, ok := prod["dimension5"].(float64); ok {
		km := int(v) * 10
		mileageKM = &km
	}
	if reg := strings.TrimSpace(fmt.Sprint(prod["dimension56"])); len(reg) >= 4 {
		if y, err := strconv.Atoi(reg[:4]); err == nil {
			year = &y
		}
	}
	dealer := "Ayvens"
	listing := schema.CarListing{
		Source:     "ayvens",
		ID:         id,
		Title:      title,
		Make:       strPtr(makeName),
		Model:      strPtr(modelName),
		Year:       year,
		MileageKM:  mileageKM,
		PriceSEK:   priceSEK,
		Fuel:       fuelLabel(fmt.Sprint(prod["dimension2"])),
		Transmission: transmissionLabel(fmt.Sprint(prod["dimension3"])),
		DealerName: &dealer,
		Raw:        map[string]any{"product": prod},
	}
	soh.Apply(&listing, "ayvens_search", title, fmt.Sprint(prod["variant"]))
	return listing
}

func parseDetailHTML(htmlBody, path string) schema.CarListing {
	id := listingIDFromPath(path)
	fields := detailFields(htmlBody)
	makeName := ""
	modelName := ""
	if name := extractRE(productNameRE, htmlBody); name != "" {
		parts := strings.Fields(name)
		if len(parts) > 0 {
			makeName = parts[0]
		}
		if len(parts) > 1 {
			modelName = strings.Join(parts[1:], " ")
		}
	}
	title := strings.TrimSpace(strings.Join([]string{makeName, modelName, extractRE(productDescRE, htmlBody)}, " "))
	var priceSEK *int
	if p := extractRE(priceContentRE, htmlBody); p != "" {
		if v, err := strconv.ParseFloat(p, 64); err == nil {
			i := int(v)
			priceSEK = &i
		}
	}
	var mileageKM, year *int
	if mil := fields["mileage"]; mil != "" {
		mileageKM = mileageFromMil(mil)
	}
	if reg := fields["registrationDate"]; reg != "" && len(reg) >= 4 {
		if y, err := strconv.Atoi(reg[:4]); err == nil {
			year = &y
		}
	}
	dealer := "Ayvens"
	pageURL := baseURL + path
	listing := schema.CarListing{
		Source:             "ayvens",
		ID:                 id,
		Title:              title,
		Make:               strPtr(makeName),
		Model:              strPtr(modelName),
		Year:               year,
		MileageKM:          mileageKM,
		PriceSEK:           priceSEK,
		Fuel:               fuelLabel(fields["fuelType"]),
		Transmission:       transmissionLabel(fields["gearType"]),
		Location:           strPtr(fields["location"]),
		DealerName:         &dealer,
		URL:                &pageURL,
		RegistrationNumber: strPtr(fields["licensePlate"]),
		Raw:                map[string]any{"listing_path": path, "details": fields},
	}
	if img := firstImageURL(htmlBody); img != "" {
		listing.ImageURL = &img
	}
	soh.Apply(&listing, "ayvens_detail", detailSOHFields(htmlBody, fields)...)
	return listing
}

func detailFields(htmlBody string) map[string]string {
	out := map[string]string{}
	for _, m := range detailContainerRE.FindAllStringSubmatch(htmlBody, -1) {
		if len(m) < 3 {
			continue
		}
		out[m[1]] = cleanText(m[2])
	}
	return out
}

func detailSOHFields(htmlBody string, fields map[string]string) []string {
	out := make([]string, 0, len(fields)+2)
	for _, v := range fields {
		out = append(out, v)
	}
	if desc := extractRE(productDescRE, htmlBody); desc != "" {
		out = append(out, desc)
	}
	return out
}

func extractInspectionURL(htmlBody string) string {
	if m := inspectionURLRE.FindStringSubmatch(htmlBody); len(m) >= 2 {
		return html.UnescapeString(m[1])
	}
	return ""
}

func firstImageURL(htmlBody string) string {
	re := regexp.MustCompile(`<img[^>]+src="(https://usedcars\.ayvens\.com/dw/image/[^"]+)"`)
	if m := re.FindStringSubmatch(htmlBody); len(m) >= 2 {
		return html.UnescapeString(strings.ReplaceAll(m[1], "&amp;", "&"))
	}
	return ""
}

func extractRE(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) >= 2 {
		return cleanText(html.UnescapeString(m[1]))
	}
	return ""
}

func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}

func mileageFromMil(s string) *int {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if digits == "" {
		return nil
	}
	mil, err := strconv.Atoi(digits)
	if err != nil {
		return nil
	}
	km := mil * 10
	return &km
}

func fuelLabel(raw string) *string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "electric", "el", "elektrisk":
		v := "El"
		return &v
	case "pluginhybrid", "plugin hybrid", "plug-in hybrid", "laddhybrid":
		v := "Laddhybrid"
		return &v
	case "hybrid":
		v := "Hybrid"
		return &v
	case "diesel":
		v := "Diesel"
		return &v
	case "gasoline", "petrol", "bensin":
		v := "Bensin"
		return &v
	default:
		if raw == "" {
			return nil
		}
		return &raw
	}
}

func transmissionLabel(raw string) *string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "automatic", "automat", "automatisk":
		v := "Automat"
		return &v
	case "manual", "manuell":
		v := "Manuell"
		return &v
	default:
		if raw == "" {
			return nil
		}
		return &raw
	}
}

func listingPath(listingID string) string {
	listingID = strings.Trim(strings.TrimSpace(listingID), "/")
	if listingID == "" {
		return ""
	}
	if strings.HasPrefix(listingID, "/sv-se/") {
		if !strings.HasSuffix(listingID, ".html") {
			listingID += ".html"
		}
		return listingID
	}
	if strings.Contains(listingID, "/") {
		return "/sv-se/" + listingID + ".html"
	}
	return ""
}

func listingIDFromPath(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	last := parts[len(parts)-1]
	return strings.TrimSuffix(last, ".html")
}

func matchesFilters(item schema.CarListing, priceMin, priceMax, mileageMax *int) bool {
	if priceMin != nil && item.PriceSEK != nil && *item.PriceSEK < *priceMin {
		return false
	}
	if priceMax != nil && item.PriceSEK != nil && *item.PriceSEK > *priceMax {
		return false
	}
	if mileageMax != nil && item.MileageKM != nil && *item.MileageKM > *mileageMax {
		return false
	}
	return true
}

func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// AttachListingPaths fills URL and listing_path from search result HTML tiles.
func AttachListingPaths(listings []schema.CarListing, htmlBody string) []schema.CarListing {
	paths := map[string]string{}
	for _, m := range tileLinkRE.FindAllStringSubmatch(htmlBody, -1) {
		if len(m) >= 3 {
			paths[m[1]] = m[2]
		}
	}
	urls := map[string]string{}
	if block := itemListRE.FindStringSubmatch(htmlBody); len(block) >= 2 {
		var data map[string]any
		if json.Unmarshal([]byte(block[1]), &data) == nil {
			items, _ := data["itemListElement"].([]any)
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					u := fmt.Sprint(m["url"])
					if u == "" {
						continue
					}
					parts := strings.Split(strings.Trim(u, "/"), "/")
					if len(parts) > 0 {
						pid := strings.TrimSuffix(parts[len(parts)-1], ".html")
						urls[pid] = u
					}
				}
			}
		}
	}
	out := make([]schema.CarListing, len(listings))
	for i, item := range listings {
		out[i] = item
		if out[i].Raw == nil {
			out[i].Raw = map[string]any{}
		}
		if p, ok := paths[item.ID]; ok {
			out[i].Raw["listing_path"] = p
			u := baseURL + p
			out[i].URL = &u
		} else if u, ok := urls[item.ID]; ok {
			out[i].URL = &u
			if idx := strings.Index(u, "/sv-se/"); idx >= 0 {
				out[i].Raw["listing_path"] = u[idx:]
			}
		}
	}
	return out
}
