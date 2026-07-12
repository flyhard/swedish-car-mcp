package repo

import (
	"path/filepath"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/aviloo/parser"
)

// FindCertInRepo finds a PDF by certificate UUID or 32-char hex download id.
func FindCertInRepo(certOrID string) (string, error) {
	key := strings.TrimSpace(certOrID)
	if key == "" {
		return "", nil
	}
	root, err := Root()
	if err != nil {
		return "", err
	}

	if hex32.MatchString(key) {
		return FindCertByStem(root, key)
	}

	if uuid.MatchString(key) {
		target := strings.ToUpper(key)
		pdfs, err := FindPDFs()
		if err != nil {
			return "", err
		}
		for _, pdf := range pdfs {
			stem := strings.TrimSuffix(filepath.Base(pdf), filepath.Ext(pdf))
			if strings.ToUpper(stem) == target {
				return pdf, nil
			}
		}
		for _, pdf := range pdfs {
			text, err := parser.ExtractText(pdf)
			if err != nil {
				continue
			}
			stem := strings.TrimSuffix(filepath.Base(pdf), filepath.Ext(pdf))
			parsed := parser.ParseAvilooText(text, &stem)
			if cert, ok := parsed["certificate_number"].(*string); ok && cert != nil && strings.ToUpper(*cert) == target {
				return pdf, nil
			}
		}
	}
	return "", nil
}
