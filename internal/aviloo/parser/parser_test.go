package parser_test

import (
	"regexp"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/aviloo/parser"
)

const sampleSnippet = `
CERTIFIKATETS NUMMER: C94D3EFA-2493-4ED3-B9FC-96DD6754575F
VARUMÄRKE: Kia
MODELL: Niro EV - 64,8 kWh
MÄTARSTÄLLNING: 57 265 km
VIN: KNACR811FP5026133
DATUM OCH TID:
2026-05-22 09:43
HÄLSOTILLSTÅND (SOH)
97,4 %
63kWh | 65kWh
448km | 460km
UTFÖRD AV: Riddermark Bil AB
UTMÄRKT HÄLSA – INGA AVVIKELSER UPPTÄCKTA
officiellt AVILOO-certifierat
`

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func TestUUIDRegex(t *testing.T) {
	if !uuidRE.MatchString("C94D3EFA-2493-4ED3-B9FC-96DD6754575F") {
		t.Fatal("expected uuid match")
	}
	if uuidRE.MatchString("not-a-uuid") {
		t.Fatal("expected no match")
	}
}

func TestSOHFromSnippet(t *testing.T) {
	downloadID := "cae9d182733848a7873983fe2b2264a0"
	parsed := parser.ParseAvilooText(sampleSnippet, &downloadID)
	soh, ok := parsed["soh_percent"].(*float64)
	if !ok || soh == nil || *soh != 97.4 {
		t.Fatalf("soh_percent = %v", parsed["soh_percent"])
	}
	cert, ok := parsed["certificate_number"].(*string)
	if !ok || cert == nil || *cert != "C94D3EFA-2493-4ED3-B9FC-96DD6754575F" {
		t.Fatalf("certificate_number = %v", parsed["certificate_number"])
	}
	vin, ok := parsed["vin"].(*string)
	if !ok || vin == nil || *vin != "KNACR811FP5026133" {
		t.Fatalf("vin = %v", parsed["vin"])
	}
	mileage, ok := parsed["mileage_km"].(*int)
	if !ok || mileage == nil || *mileage != 57265 {
		t.Fatalf("mileage_km = %v", parsed["mileage_km"])
	}
	if certified, ok := parsed["certified"].(bool); !ok || !certified {
		t.Fatalf("certified = %v", parsed["certified"])
	}
}
