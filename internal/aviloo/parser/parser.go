package parser

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

var (
	uuidRE     = regexp.MustCompile(`([0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12})`)
	sohLabelRE = regexp.MustCompile(`(?i)H[\xC4A]LSOTILLST[\xC4A]ND\s*\(SOH\)[\s\S]{0,400}?(\d{1,3}(?:[.,]\d+)?)\s*%`)
	percentRE  = regexp.MustCompile(`(\d{1,3}(?:[.,]\d+)?)\s*%`)
	energyRE   = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*kWh\s*\|\s*(\d+(?:[.,]\d+)?)\s*kWh`)
	wltpRE     = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*km\s*\|\s*(\d+(?:[.,]\d+)?)\s*km`)
)

// Result holds parsed AVILOO certificate fields.
type Result map[string]any

// ExtractText reads plain text from a PDF using pdftotext when available, else a Go fallback.
func ExtractText(pdfPath string) (string, error) {
	if exe, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command(exe, pdfPath, "-").Output()
		if err != nil {
			return "", fmt.Errorf("pdftotext failed: %w", err)
		}
		return string(out), nil
	}
	return pdfTextFallback(pdfPath)
}

func pdfTextFallback(pdfPath string) (string, error) {
	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		return "", fmt.Errorf("pdftotext missing and pdf fallback failed: %w", err)
	}
	defer f.Close()
	var buf bytes.Buffer
	total := r.NumPage()
	for i := 1; i <= total; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf.WriteString(text)
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

func parseFloat(s string) float64 {
	s = strings.ReplaceAll(strings.ReplaceAll(s, ",", "."), " ", "")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseIntKM(s string) int {
	digits := regexp.MustCompile(`\d`).FindAllString(s, -1)
	if len(digits) == 0 {
		return 0
	}
	v, _ := strconv.Atoi(strings.Join(digits, ""))
	return v
}

func fieldAfter(text, labelPattern string) *string {
	re := regexp.MustCompile("(?i)" + labelPattern + `\s*(.+)`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return nil
	}
	line := strings.TrimSpace(strings.Split(m[1], "\n")[0])
	if line == "" {
		return nil
	}
	return &line
}

// ParseAvilooText extracts certificate fields from extracted PDF text.
func ParseAvilooText(text string, downloadID *string) Result {
	rawSOH := make([]string, 0)
	for _, m := range percentRE.FindAllStringSubmatch(text, -1) {
		rawSOH = append(rawSOH, m[0])
	}

	var certificateNumber *string
	certLine := regexp.MustCompile(`(?i)CERTIFIKATETS\s+NUMMER:\s*(` + uuidRE.String() + `)`).FindStringSubmatch(text)
	if len(certLine) >= 2 {
		upper := strings.ToUpper(certLine[1])
		certificateNumber = &upper
	} else if m := uuidRE.FindStringSubmatch(text); len(m) >= 2 {
		upper := strings.ToUpper(m[1])
		certificateNumber = &upper
	}

	brand := fieldAfter(text, `VARUM[\xC4A]RKE:`)
	model := fieldAfter(text, `MODELL:`)
	vin := fieldAfter(text, `VIN:`)
	mileageRaw := fieldAfter(text, `M[\xC4A]TARST[\xC4A]LLNING:`)
	var mileageKM *int
	if mileageRaw != nil {
		v := parseIntKM(*mileageRaw)
		mileageKM = &v
	}

	var testedAt *string
	if dt := regexp.MustCompile(`(?i)DATUM\s+OCH\s+TID:\s*\n?\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2})`).FindStringSubmatch(text); len(dt) >= 2 {
		v := strings.TrimSpace(dt[1])
		testedAt = &v
	}

	performedBy := fieldAfter(text, `UTF[\xD6O]RD\s+AV:`)

	var sohPercent *float64
	if sohM := sohLabelRE.FindStringSubmatch(text); len(sohM) >= 2 {
		v := parseFloat(sohM[1])
		sohPercent = &v
	} else {
		for _, pm := range percentRE.FindAllStringSubmatch(text, -1) {
			val := parseFloat(pm[1])
			if val >= 50 && val <= 100 {
				sohPercent = &val
				break
			}
		}
	}

	var energyCurrent, energyNew *float64
	if em := energyRE.FindStringSubmatch(text); len(em) >= 3 {
		c := parseFloat(em[1])
		n := parseFloat(em[2])
		energyCurrent = &c
		energyNew = &n
	}

	var wltpCurrent, wltpNew *float64
	if wm := wltpRE.FindStringSubmatch(text); len(wm) >= 3 {
		c := parseFloat(wm[1])
		n := parseFloat(wm[2])
		wltpCurrent = &c
		wltpNew = &n
	}

	var assessment *string
	if regexp.MustCompile(`(?i)UTM[\xC4A]RKT\s+H[\xC4A]LSA`).MatchString(text) {
		v := "UTMÄRKT HÄLSA"
		assessment = &v
	}

	certified := regexp.MustCompile(`(?i)AVILOO-certifierat`).MatchString(text) ||
		regexp.MustCompile(`(?i)officiellt\s+AVILOO`).MatchString(text)

	return Result{
		"certificate_number": certificateNumber,
		"download_id":        downloadID,
		"soh_percent":        sohPercent,
		"vin":                vin,
		"mileage_km":         mileageKM,
		"tested_at":          testedAt,
		"brand":              brand,
		"model":              model,
		"performed_by":       performedBy,
		"assessment":         assessment,
		"certified":          certified,
		"energy_current_kwh": energyCurrent,
		"energy_new_kwh":     energyNew,
		"wltp_current_km":    wltpCurrent,
		"wltp_new_km":        wltpNew,
		"raw_soh_matches":    rawSOH,
	}
}

// ParsePDF extracts and parses a PDF file.
func ParsePDF(pdfPath string) (Result, error) {
	text, err := ExtractText(pdfPath)
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	return ParseAvilooText(text, &stem), nil
}
