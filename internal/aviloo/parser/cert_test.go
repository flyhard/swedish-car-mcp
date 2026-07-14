package parser_test

import (
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/aviloo/parser"
)

func TestDetectCertType(t *testing.T) {
	if got := parser.DetectCertType("officiellt AVILOO-certifierat"); got != parser.CertTypeAviloo {
		t.Fatalf("got %q", got)
	}
	if got := parser.DetectCertType("SoHSCAN batteritest 94%"); got != parser.CertTypeSoHSCAN {
		t.Fatalf("got %q", got)
	}
	if got := parser.DetectCertType("DEKRA batterihälsa"); got != parser.CertTypeDEKRA {
		t.Fatalf("got %q", got)
	}
}

func TestExtractCertSoHSCANFallback(t *testing.T) {
	parsed := parser.ParseAvilooText("SoHSCAN batterihälsa 93,2 %", nil)
	parsed["cert_type"] = string(parser.DetectCertType("SoHSCAN batterihälsa 93,2 %"))
	if soh, ok := parsed["soh_percent"].(*float64); !ok || soh == nil {
		t.Fatalf("missing soh in %+v", parsed)
	}
}
