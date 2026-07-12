package soh

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
)

var (
	sohPercentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:soh|hälsotillstånd|batterihälsa|state\s+of\s+health)[^\d]{0,20}(\d{1,3}(?:[.,]\d{1,2})?)\s*%`),
		regexp.MustCompile(`(?i)(\d{1,3}(?:[.,]\d{1,2})?)\s*%\s*(?:soh|batterihälsa|hälsotillstånd)`),
		regexp.MustCompile(`(?i)(\d{1,3}(?:[.,]\d{1,2})?)\s*%\s*(?:batteri)?hälsa`),
		regexp.MustCompile(`(?i)hälsotillstånd\s*\(soh\)[^\d]{0,20}(\d{1,3}(?:[.,]\d{1,2})?)\s*%`),
		regexp.MustCompile(`(?i)pro\s+soh\s+(\d{1,3}(?:[.,]\d{1,2})?)\s*%`),
	}
	batteryTestedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bbatteritestad\b`),
		regexp.MustCompile(`(?i)\baviloo\b`),
		regexp.MustCompile(`(?i)\bhälsotillstånd\b`),
		regexp.MustCompile(`(?i)\bsoh\b`),
		regexp.MustCompile(`(?i)\bbatterihälsa\b`),
	}
)

// Result holds SoH extraction output.
type Result struct {
	SOHPercent    *float64
	BatteryTested bool
	SOHRawMatch   *string
}

func normalizePercent(raw string) *float64 {
	raw = strings.ReplaceAll(raw, ",", ".")
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || v > 100 {
		return nil
	}
	return &v
}

// ExtractFromText parses SoH hints from free text.
func ExtractFromText(text string) Result {
	result := Result{}
	if strings.TrimSpace(text) == "" {
		return result
	}
	for _, p := range batteryTestedPatterns {
		if p.MatchString(text) {
			result.BatteryTested = true
			break
		}
	}
	for _, p := range sohPercentPatterns {
		m := p.FindStringSubmatch(text)
		if len(m) >= 2 {
			if pct := normalizePercent(m[1]); pct != nil {
				result.SOHPercent = pct
				match := m[0]
				result.SOHRawMatch = &match
				return result
			}
		}
	}
	return result
}

// ExtractFromFields combines SoH extraction across multiple text fields.
func ExtractFromFields(fields ...string) Result {
	combined := Result{}
	for _, field := range fields {
		if field == "" {
			continue
		}
		found := ExtractFromText(field)
		if found.BatteryTested {
			combined.BatteryTested = true
		}
		if combined.SOHPercent == nil && found.SOHPercent != nil {
			combined.SOHPercent = found.SOHPercent
			combined.SOHRawMatch = found.SOHRawMatch
		}
	}
	return combined
}

// Apply mutates listing with SoH data when found.
func Apply(listing *schema.CarListing, source string, fields ...string) {
	soh := ExtractFromFields(fields...)
	if soh.SOHPercent == nil && !soh.BatteryTested {
		return
	}
	if soh.SOHPercent != nil {
		listing.SOHPercent = soh.SOHPercent
		listing.SOHRawMatch = soh.SOHRawMatch
		listing.SOHSource = schema.StrPtr(source)
	}
	if soh.BatteryTested {
		listing.BatteryTested = true
	}
}
