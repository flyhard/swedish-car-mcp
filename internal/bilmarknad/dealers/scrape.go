package dealers

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/ayvens"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/riddermark"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

var (
	uuidRE     = regexp.MustCompile(`[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}`)
	hex32RE    = regexp.MustCompile(`[0-9a-fA-F]{32}`)
	rideBlobRE = regexp.MustCompile(`https://ride\.blob\.core\.windows\.net/battery-tests/[0-9a-fA-F]+\.pdf`)
	busURLRE   = regexp.MustCompile(`https://bus2\.bus\.no[^\s"'<>]+`)
	pdfURLRE   = regexp.MustCompile(`https?://[^\s"'<>]+\.pdf`)
	accRE      = regexp.MustCompile(`(?i)\b(adaptive cruise|acc|adaptiv farthållare)\b`)
)

// Signals holds battery and ACC hints scraped from a dealer page.
type Signals struct {
	URL           string   `json:"url"`
	Dealer        string   `json:"dealer"`
	SOHPercent    *float64 `json:"soh_percent,omitempty"`
	BatteryTested bool     `json:"battery_tested"`
	SOHRawMatch   *string  `json:"soh_raw_match,omitempty"`
	CertURLs      []string `json:"cert_urls,omitempty"`
	CertIDs       []string `json:"cert_ids,omitempty"`
	BUSLinks      []string `json:"bus_links,omitempty"`
	ACCMentioned  bool     `json:"acc_mentioned"`
	TextSnippets  []string `json:"text_snippets,omitempty"`
}

// ScrapePage fetches a dealer listing URL and extracts battery/ACC signals.
func ScrapePage(ctx context.Context, pageURL string) (*Signals, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	source, id, ok := urls.ParseListingURL(pageURL)
	if !ok {
		return scrapeGeneric(ctx, pageURL)
	}
	switch source {
	case "riddermark":
		client := riddermark.NewClient(nil)
		defer client.Close()
		item, err := client.GetListing(ctx, id)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return &Signals{URL: pageURL, Dealer: "riddermark"}, nil
		}
		return signalsFromListing(pageURL, "riddermark", item.Title, item.SOHPercent, item.BatteryTested, item.SOHRawMatch, listingFields(item.Raw)), nil
	case "ayvens":
		client := ayvens.NewClient(nil)
		defer client.Close()
		item, err := client.GetListing(ctx, id)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return &Signals{URL: pageURL, Dealer: "ayvens"}, nil
		}
		fields := listingFields(item.Raw)
		if report, ok := item.Raw["inspection_report_url"].(string); ok && report != "" {
			fields = append(fields, report)
		}
		return signalsFromListing(pageURL, "ayvens", item.Title, item.SOHPercent, item.BatteryTested, item.SOHRawMatch, fields), nil
	default:
		return scrapeGeneric(ctx, pageURL)
	}
}

func scrapeGeneric(ctx context.Context, pageURL string) (*Signals, error) {
	client := httputil.NewRedirectClient(map[string]string{"User-Agent": httputil.UserAgent})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httputil.DoWithRetry(ctx, client, req, "dealer", httputil.DefaultRetryPolicy())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	text := html.UnescapeString(string(body))
	dealer := guessDealer(pageURL)
	found := soh.ExtractFromText(text)
	out := &Signals{
		URL:           pageURL,
		Dealer:        dealer,
		SOHPercent:    found.SOHPercent,
		BatteryTested: found.BatteryTested,
		SOHRawMatch:   found.SOHRawMatch,
		ACCMentioned:  accRE.MatchString(text),
	}
	out.CertURLs = unique(extractAll(pdfURLRE, text))
	out.CertURLs = append(out.CertURLs, unique(extractAll(rideBlobRE, text))...)
	out.BUSLinks = unique(extractAll(busURLRE, text))
	out.CertIDs = uniqueIDs(text)
	if found.SOHRawMatch != nil {
		out.TextSnippets = append(out.TextSnippets, *found.SOHRawMatch)
	}
	return out, nil
}

func signalsFromListing(pageURL, dealer, title string, sohPct *float64, tested bool, rawMatch *string, fields []string) *Signals {
	text := strings.Join(fields, "\n")
	if title != "" {
		text = title + "\n" + text
	}
	found := soh.ExtractFromFields(append(fields, title)...)
	out := &Signals{
		URL:           pageURL,
		Dealer:        dealer,
		SOHPercent:    sohPct,
		BatteryTested: tested,
		SOHRawMatch:   rawMatch,
		ACCMentioned:  accRE.MatchString(text),
	}
	if out.SOHPercent == nil {
		out.SOHPercent = found.SOHPercent
		out.SOHRawMatch = found.SOHRawMatch
	}
	if !out.BatteryTested {
		out.BatteryTested = found.BatteryTested
	}
	out.CertURLs = unique(extractAll(pdfURLRE, text))
	out.CertURLs = append(out.CertURLs, unique(extractAll(rideBlobRE, text))...)
	out.BUSLinks = unique(extractAll(busURLRE, text))
	out.CertIDs = uniqueIDs(text)
	return out
}

func listingFields(raw map[string]any) []string {
	if raw == nil {
		return nil
	}
	var fields []string
	for _, v := range raw {
		fields = append(fields, fmt.Sprint(v))
	}
	return fields
}

func guessDealer(pageURL string) string {
	lower := strings.ToLower(pageURL)
	switch {
	case strings.Contains(lower, "riddermark"):
		return "riddermark"
	case strings.Contains(lower, "ayvens"), strings.Contains(lower, "leaseplan"):
		return "ayvens"
	case strings.Contains(lower, "dinbil"), strings.Contains(lower, "din-bil"):
		return "din_bil"
	default:
		return "unknown"
	}
}

func extractAll(re *regexp.Regexp, text string) []string {
	return re.FindAllString(text, -1)
}

func uniqueIDs(text string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range uuidRE.FindAllString(text, -1) {
		key := strings.ToUpper(m)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	for _, m := range hex32RE.FindAllString(text, -1) {
		key := strings.ToLower(m)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func unique(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
