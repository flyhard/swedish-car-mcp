package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	hex32 = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	uuid  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// Root returns the configured repository root.
func Root() (string, error) {
	if env := os.Getenv("AVILOO_MCP_REPO_ROOT"); env != "" {
		return filepath.Abs(env)
	}
	return os.Getwd()
}

// AssertInRepo resolves path and ensures it stays under the repo root.
func AssertInRepo(path string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path is outside repository: %s", abs)
	}
	return abs, nil
}

func looksLikeAvilooPDF(root, abs string) bool {
	nameLower := strings.ToLower(filepath.Base(abs))
	if strings.Contains(nameLower, "aviloo") {
		return true
	}
	stem := strings.TrimSuffix(nameLower, filepath.Ext(nameLower))
	if hex32.MatchString(stem) {
		return true
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if part == "docs" && strings.HasSuffix(strings.ToLower(abs), ".pdf") {
			return true
		}
	}
	if filepath.Dir(abs) == root && strings.HasSuffix(strings.ToLower(abs), ".pdf") {
		return true
	}
	return false
}

// FindPDFs lists PDF paths in the repo that may be AVILOO certificates.
func FindPDFs() ([]string, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	var candidates []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".pdf") {
			return nil
		}
		if _, err := AssertInRepo(path); err != nil {
			return nil
		}
		if looksLikeAvilooPDF(root, path) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		ri, _ := filepath.Rel(root, candidates[i])
		rj, _ := filepath.Rel(root, candidates[j])
		return ri < rj
	})
	return candidates, nil
}

// FindPDFsForRegnr returns PDFs whose path contains the registration number.
func FindPDFsForRegnr(regnr string) ([]string, error) {
	regnr = strings.ToUpper(strings.TrimSpace(regnr))
	if regnr == "" {
		return nil, nil
	}
	pdfs, err := FindPDFs()
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, pdf := range pdfs {
		rel := strings.ToUpper(Rel(pdf))
		if strings.Contains(rel, regnr) || strings.Contains(strings.ToUpper(filepath.Base(pdf)), regnr) {
			matches = append(matches, pdf)
		}
	}
	return matches, nil
}

// Rel returns a path relative to the repo root when possible.
func Rel(abs string) string {
	root, err := Root()
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return rel
}

// CertFinder resolves certificate PDFs by UUID or hex download id.
type CertFinder interface {
	FindCertInRepo(certOrID string) (string, error)
}

// FindCertByStem looks up a PDF by 32-char hex filename stem.
func FindCertByStem(root, key string) (string, error) {
	pdfs, err := FindPDFs()
	if err != nil {
		return "", err
	}
	keyLower := strings.ToLower(key)
	for _, pdf := range pdfs {
		stem := strings.TrimSuffix(filepath.Base(pdf), filepath.Ext(pdf))
		if strings.ToLower(stem) == keyLower {
			return pdf, nil
		}
	}
	direct := filepath.Join(root, keyLower+".pdf")
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return AssertInRepo(direct)
	}
	var matches []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), keyLower+".pdf") {
			if abs, err := AssertInRepo(path); err == nil {
				matches = append(matches, abs)
			}
		}
		return nil
	})
	if len(matches) > 0 {
		return matches[0], nil
	}
	return "", nil
}
