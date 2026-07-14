package cert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/flyhard/swedish-car-mcp/internal/aviloo/parser"
	"github.com/flyhard/swedish-car-mcp/internal/aviloo/repo"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/bus"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

var (
	uuidRE      = regexp.MustCompile(`[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}`)
	hex32RE     = regexp.MustCompile(`[0-9a-fA-F]{32}`)
	busURLRE    = regexp.MustCompile(`https://bus2\.bus\.no[^\s"'<>]+`)
	rideBlobRE  = regexp.MustCompile(`https://ride\.blob\.core\.windows\.net/battery-tests/[0-9a-fA-F]+\.pdf`)
	pdfURLRE    = regexp.MustCompile(`https?://[^\s"'<>]+\.pdf`)
)

// Download saves a certificate PDF under the configured repo root.
func Download(ctx context.Context, pdfURL, destPath string) (string, error) {
	root, err := repo.Root()
	if err != nil {
		return "", err
	}
	abs, err := repo.AssertInRepo(filepath.Join(root, destPath))
	if err != nil {
		abs, err = repo.AssertInRepo(destPath)
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	data, err := bus.DownloadPDF(ctx, nil, pdfURL)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}
	return repo.Rel(abs), nil
}

// DefaultDestPath builds docs/modeller/{model}/annonser/{regnr}/{filename}.
func DefaultDestPath(model, regnr, filename string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	regnr = strings.ToUpper(strings.TrimSpace(regnr))
	if filename == "" {
		filename = fmt.Sprintf("%s-battery-%s.pdf", regnr, time.Now().Format("2006-01-02"))
	}
	return filepath.Join("docs", "modeller", model, "annonser", strings.ToLower(regnr), filename)
}

// HuntResult summarizes a SoH hunt for one registration number.
type HuntResult struct {
	RegNr           string           `json:"regnr"`
	RepoPDFs        []string         `json:"repo_pdfs,omitempty"`
	Listing         map[string]any   `json:"listing,omitempty"`
	CertURLs        []string         `json:"cert_urls,omitempty"`
	CertIDs         []string         `json:"cert_ids,omitempty"`
	BUSLinks        []string         `json:"bus_links,omitempty"`
	LookupResults   []map[string]any `json:"lookup_results,omitempty"`
	BUSReports      []map[string]any `json:"bus_reports,omitempty"`
	DownloadedPDF   *string          `json:"downloaded_pdf,omitempty"`
	Extracted       map[string]any   `json:"extracted,omitempty"`
	SOHPercent      *float64         `json:"soh_percent,omitempty"`
	SOHSource       *string          `json:"soh_source,omitempty"`
	BatteryTested   bool             `json:"battery_tested"`
	Steps           []string         `json:"steps"`
}

// Hunt orchestrates repo → listing → dealer signals → cert lookup for one reg.nr.
func Hunt(ctx context.Context, regnr string, model *string, download bool) (*HuntResult, error) {
	regnr = schema.NormalizeRegistrationNumber(regnr)
	if regnr == "" {
		return nil, fmt.Errorf("regnr is required")
	}
	out := &HuntResult{RegNr: regnr, Steps: []string{}}

	pdfs, err := repo.FindPDFsForRegnr(regnr)
	if err != nil {
		return nil, err
	}
	if len(pdfs) > 0 {
		out.Steps = append(out.Steps, "repo_pdfs")
		for _, p := range pdfs {
			out.RepoPDFs = append(out.RepoPDFs, repo.Rel(p))
		}
		parsed, err := parser.ExtractCert(pdfs[0])
		if err == nil {
			out.Extracted = parsed
			applyParsed(out, parsed, strPtr("repo"))
			if out.SOHPercent != nil {
				return out, nil
			}
		}
	}

	svc := &search.Service{}
	item := svc.GetListing(ctx, nil, &regnr, nil)
	if item != nil {
		out.Steps = append(out.Steps, "listing")
		out.Listing = item
		text := listingText(item)
		out.CertURLs = append(out.CertURLs, uniqueStrings(extractMatches(pdfURLRE, text))...)
		out.CertURLs = append(out.CertURLs, uniqueStrings(extractMatches(rideBlobRE, text))...)
		out.BUSLinks = uniqueStrings(extractMatches(busURLRE, text))
		out.CertIDs = uniqueIDs(text)
		if sohPct, ok := item["soh_percent"].(float64); ok && sohPct > 0 {
			out.SOHPercent = &sohPct
			src := "listing"
			out.SOHSource = &src
			out.BatteryTested = true
		} else if tested, ok := item["battery_tested"].(bool); ok && tested {
			out.BatteryTested = true
			if raw, ok := item["soh_raw_match"].(string); ok {
				found := soh.ExtractFromText(raw)
				out.SOHPercent = found.SOHPercent
				if found.SOHPercent != nil {
					src := "listing_text"
					out.SOHSource = &src
				}
			}
		}
		if dealerURL := dealerPageURL(item); dealerURL != "" {
			out.CertURLs = append(out.CertURLs, dealerURL)
		}
	}

	for _, id := range out.CertIDs {
		found, err := repo.FindCertInRepo(id)
		if err != nil {
			continue
		}
		if found == "" {
			out.Steps = append(out.Steps, "lookup_miss:"+id)
			continue
		}
		parsed, err := parser.ExtractCert(found)
		entry := map[string]any{"cert_id": id, "pdf_path": repo.Rel(found), "found": true}
		if err == nil {
			entry["parsed"] = parsed
			if out.Extracted == nil {
				out.Extracted = parsed
			}
			applyParsed(out, parsed, strPtr("lookup"))
		}
		out.LookupResults = append(out.LookupResults, entry)
		if out.SOHPercent != nil {
			break
		}
	}

	for _, busURL := range out.BUSLinks {
		client := bus.NewClient(nil)
		report, err := client.FetchReport(ctx, busURL)
		if err != nil {
			out.BUSReports = append(out.BUSReports, map[string]any{"url": busURL, "error": err.Error()})
			continue
		}
		entry := map[string]any{
			"url":                  busURL,
			"sales_report_id":      report.SalesReportID,
			"test_id":              report.TestID,
			"aviloo_battery_score": report.AvilooBatteryScore,
			"battery_result":       report.BatteryResult,
			"aviloo_report_url":    report.AvilooReportURL,
		}
		out.BUSReports = append(out.BUSReports, entry)
		out.Steps = append(out.Steps, "bus_report")
		if pct := bus.BestSOH(report); pct != nil && out.SOHPercent == nil {
			out.SOHPercent = pct
			src := "bus_dekra"
			out.SOHSource = &src
			out.BatteryTested = true
		}
		if download && report.AvilooReportURL != nil && model != nil && *model != "" {
			dest := DefaultDestPath(*model, regnr, "")
			if saved, err := Download(ctx, *report.AvilooReportURL, dest); err == nil {
				out.DownloadedPDF = &saved
				if parsed, err := parser.ExtractCert(filepath.Join(mustRoot(), saved)); err == nil {
					out.Extracted = parsed
					applyParsed(out, parsed, strPtr("bus_pdf"))
				}
			}
		}
	}

	if download && out.SOHPercent == nil {
		for _, u := range out.CertURLs {
			if !rideBlobRE.MatchString(u) && !strings.Contains(strings.ToLower(u), "aviloo") {
				continue
			}
			if model == nil || *model == "" {
				break
			}
			dest := DefaultDestPath(*model, regnr, filepath.Base(u))
			if saved, err := Download(ctx, u, dest); err == nil {
				out.DownloadedPDF = &saved
				out.Steps = append(out.Steps, "downloaded_cert")
				if parsed, err := parser.ExtractCert(filepath.Join(mustRoot(), saved)); err == nil {
					out.Extracted = parsed
					applyParsed(out, parsed, strPtr("download"))
				}
				break
			}
		}
	}

	return out, nil
}

func mustRoot() string {
	root, _ := repo.Root()
	return root
}

func applyParsed(out *HuntResult, parsed parser.Result, source *string) {
	if sohPct, ok := parsed["soh_percent"].(*float64); ok && sohPct != nil {
		out.SOHPercent = sohPct
		out.SOHSource = source
		out.BatteryTested = true
	}
	if certified, ok := parsed["certified"].(bool); ok && certified {
		out.BatteryTested = true
	}
}

func listingText(item map[string]any) string {
	var parts []string
	for _, key := range []string{"title", "description", "short_description", "long_description"} {
		if v, ok := item[key]; ok {
			parts = append(parts, fmt.Sprint(v))
		}
	}
	if raw, ok := item["raw"].(map[string]any); ok {
		for _, v := range raw {
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return strings.Join(parts, "\n")
}

func dealerPageURL(item map[string]any) string {
	if u, ok := item["url"].(string); ok && u != "" {
		return u
	}
	return ""
}

func extractMatches(re *regexp.Regexp, text string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range re.FindAllString(text, -1) {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
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

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func strPtr(s string) *string { return &s }
