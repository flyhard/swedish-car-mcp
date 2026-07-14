package parser

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

// CertType identifies the certificate format.
type CertType string

const (
	CertTypeAviloo  CertType = "aviloo"
	CertTypeDEKRA   CertType = "dekra"
	CertTypeSoHSCAN CertType = "sohscan"
	CertTypeUnknown CertType = "unknown"
)

var (
	dekraRE    = regexp.MustCompile(`(?i)\bDEKRA\b`)
	sohscanRE  = regexp.MustCompile(`(?i)\bSoHSCAN\b`)
	avilooRE   = regexp.MustCompile(`(?i)\bAVILOO\b`)
)

// DetectCertType classifies extracted PDF text.
func DetectCertType(text string) CertType {
	upper := strings.ToUpper(text)
	switch {
	case avilooRE.MatchString(text) || strings.Contains(upper, "CERTIFIKATETS NUMMER"):
		return CertTypeAviloo
	case sohscanRE.MatchString(text):
		return CertTypeSoHSCAN
	case dekraRE.MatchString(text):
		return CertTypeDEKRA
	default:
		return CertTypeUnknown
	}
}

// ExtractCert parses any supported battery certificate PDF.
func ExtractCert(pdfPath string) (Result, error) {
	text, err := ExtractText(pdfPath)
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	certType := DetectCertType(text)
	out := ParseAvilooText(text, &stem)
	out["cert_type"] = string(certType)

	if certType == CertTypeSoHSCAN || certType == CertTypeUnknown {
		if sohPct, ok := out["soh_percent"].(*float64); !ok || sohPct == nil {
			sohResult := soh.ExtractFromText(text)
			if sohResult.SOHPercent != nil {
				out["soh_percent"] = sohResult.SOHPercent
				out["soh_raw_match"] = sohResult.SOHRawMatch
			}
			if sohResult.BatteryTested {
				out["battery_tested"] = true
			}
		}
	}
	if certType == CertTypeDEKRA && out["soh_percent"] == nil {
		sohResult := soh.ExtractFromText(text)
		if sohResult.SOHPercent != nil {
			out["soh_percent"] = sohResult.SOHPercent
		}
	}
	return out, nil
}
