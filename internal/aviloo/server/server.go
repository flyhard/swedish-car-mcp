package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/aviloo/cert"
	"github.com/flyhard/swedish-car-mcp/internal/aviloo/parser"
	"github.com/flyhard/swedish-car-mcp/internal/aviloo/repo"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/bus"
	"github.com/flyhard/swedish-car-mcp/internal/mcpjson"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.3.0"

type extractPDFInput struct {
	PDFPath string `json:"pdf_path" jsonschema:"relative or absolute path under repo root,required"`
}

type lookupCertInput struct {
	CertOrID string `json:"cert_or_id" jsonschema:"certificate UUID or 32-char hex download id,required"`
}

type fetchBUSInput struct {
	ReportURL string `json:"report_url" jsonschema:"BUS/DEKRA salesReportLink URL,required"`
}

type downloadCertInput struct {
	URL      string  `json:"url" jsonschema:"PDF download URL,required"`
	DestPath *string `json:"dest_path" jsonschema:"relative path under repo root"`
	Model    *string `json:"model" jsonschema:"model slug for default dest path"`
	RegNr    *string `json:"regnr" jsonschema:"registration number for default dest path"`
	Filename *string `json:"filename" jsonschema:"optional filename when using model+regnr"`
}

type huntSOHInput struct {
	RegNr    string  `json:"regnr" jsonschema:"registration number,required"`
	Model    *string `json:"model" jsonschema:"model slug for downloads"`
	Download *bool   `json:"download" jsonschema:"download discovered PDFs when true"`
}

func extractAvilooPDF(_ context.Context, _ *mcp.CallToolRequest, in extractPDFInput) (*mcp.CallToolResult, struct{}, error) {
	return extractCertPDF(in.PDFPath)
}

func extractCert(_ context.Context, _ *mcp.CallToolRequest, in extractPDFInput) (*mcp.CallToolResult, struct{}, error) {
	return extractCertPDF(in.PDFPath)
}

func extractCertPDF(pdfPath string) (*mcp.CallToolResult, struct{}, error) {
	root, err := repo.Root()
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	candidate := pdfPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	resolved, err := repo.AssertInRepo(candidate)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return mcpjson.ErrorResult(fmt.Sprintf("file not found: %s", repo.Rel(resolved)))
	}
	data, err := parser.ExtractCert(resolved)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	data["pdf_path"] = repo.Rel(resolved)
	return mcpjson.TextResult(data)
}

func lookupAvilooCert(_ context.Context, _ *mcp.CallToolRequest, in lookupCertInput) (*mcp.CallToolResult, struct{}, error) {
	found, err := repo.FindCertInRepo(in.CertOrID)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	if found == "" {
		return mcpjson.TextResult(map[string]any{"found": false, "cert_or_id": in.CertOrID})
	}
	data, err := parser.ExtractCert(found)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	data["found"] = true
	data["pdf_path"] = repo.Rel(found)
	return mcpjson.TextResult(data)
}

func listAvilooPDFs(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, struct{}, error) {
	root, err := repo.Root()
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	pdfs, err := repo.FindPDFs()
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	rel := make([]string, len(pdfs))
	for i, p := range pdfs {
		rel[i] = repo.Rel(p)
	}
	return mcpjson.TextResult(map[string]any{
		"repo_root": root,
		"count":     len(rel),
		"pdfs":      rel,
	})
}

func fetchBUSDEKRA(ctx context.Context, _ *mcp.CallToolRequest, in fetchBUSInput) (*mcp.CallToolResult, struct{}, error) {
	client := bus.NewClient(nil)
	report, err := client.FetchReport(ctx, in.ReportURL)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	out := map[string]any{
		"report":     report,
		"soh_percent": bus.BestSOH(report),
		"cert_type":  "dekra",
	}
	return mcpjson.TextResult(out)
}

func downloadCertPDF(ctx context.Context, _ *mcp.CallToolRequest, in downloadCertInput) (*mcp.CallToolResult, struct{}, error) {
	dest := ""
	if in.DestPath != nil && strings.TrimSpace(*in.DestPath) != "" {
		dest = strings.TrimSpace(*in.DestPath)
	} else if in.Model != nil && in.RegNr != nil && *in.Model != "" && *in.RegNr != "" {
		filename := ""
		if in.Filename != nil {
			filename = *in.Filename
		}
		dest = cert.DefaultDestPath(*in.Model, *in.RegNr, filename)
	} else {
		return mcpjson.ErrorResult("dest_path or model+regnr required")
	}
	saved, err := cert.Download(ctx, in.URL, dest)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	root, _ := repo.Root()
	parsed, parseErr := parser.ExtractCert(filepath.Join(root, saved))
	result := map[string]any{"pdf_path": saved, "dest_path": dest}
	if parseErr == nil {
		result["parsed"] = parsed
	}
	return mcpjson.TextResult(result)
}

func huntSOH(ctx context.Context, _ *mcp.CallToolRequest, in huntSOHInput) (*mcp.CallToolResult, struct{}, error) {
	download := in.Download != nil && *in.Download
	result, err := cert.Hunt(ctx, in.RegNr, in.Model, download)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	return mcpjson.TextResult(result)
}

// Run starts the aviloo MCP server on stdio.
func Run(ctx context.Context) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aviloo", Version: version}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "extract_aviloo_pdf", Description: "Extract AVILOO certificate fields from a PDF in the repo."}, extractAvilooPDF)
	mcp.AddTool(srv, &mcp.Tool{Name: "extract_cert", Description: "Unified parser for AVILOO, DEKRA, and SoHSCAN certificate PDFs."}, extractCert)
	mcp.AddTool(srv, &mcp.Tool{Name: "lookup_aviloo_cert", Description: "Find and parse an AVILOO certificate by UUID or download id."}, lookupAvilooCert)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_aviloo_pdfs", Description: "List AVILOO-like PDF files in the configured repo."}, listAvilooPDFs)
	mcp.AddTool(srv, &mcp.Tool{Name: "fetch_bus_dekra", Description: "Resolve BUS/DEKRA salesReportLink to SoH % and optional AVILOO PDF URL."}, fetchBUSDEKRA)
	mcp.AddTool(srv, &mcp.Tool{Name: "download_cert_pdf", Description: "Download a battery certificate PDF into docs/modeller/{model}/annonser/{regnr}/."}, downloadCertPDF)
	mcp.AddTool(srv, &mcp.Tool{Name: "hunt_soh", Description: "Orchestrate SoH hunt: repo PDFs, listing body, BUS/AVILOO links for one reg.nr."}, huntSOH)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
