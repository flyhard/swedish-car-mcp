package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/aviloo/parser"
	"github.com/flyhard/swedish-car-mcp/internal/aviloo/repo"
	"github.com/flyhard/swedish-car-mcp/internal/mcpjson"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.2.0"

type extractPDFInput struct {
	PDFPath string `json:"pdf_path" jsonschema:"relative or absolute path under repo root,required"`
}

type lookupCertInput struct {
	CertOrID string `json:"cert_or_id" jsonschema:"certificate UUID or 32-char hex download id,required"`
}

func extractAvilooPDF(_ context.Context, _ *mcp.CallToolRequest, in extractPDFInput) (*mcp.CallToolResult, struct{}, error) {
	root, err := repo.Root()
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	candidate := in.PDFPath
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
	text, err := parser.ExtractText(resolved)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	stem := strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
	data := parser.ParseAvilooText(text, &stem)
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
	text, err := parser.ExtractText(found)
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	stem := strings.TrimSuffix(filepath.Base(found), filepath.Ext(found))
	data := parser.ParseAvilooText(text, &stem)
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

// Run starts the aviloo MCP server on stdio.
func Run(ctx context.Context) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aviloo", Version: version}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "extract_aviloo_pdf", Description: "Extract AVILOO certificate fields from a PDF in the repo."}, extractAvilooPDF)
	mcp.AddTool(srv, &mcp.Tool{Name: "lookup_aviloo_cert", Description: "Find and parse an AVILOO certificate by UUID or download id."}, lookupAvilooCert)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_aviloo_pdfs", Description: "List AVILOO-like PDF files in the configured repo."}, listAvilooPDFs)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
