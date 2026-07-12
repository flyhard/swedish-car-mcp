package soh_test

import (
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

func TestExtractSOHPercentFromSOHLabel(t *testing.T) {
	result := soh.ExtractFromText("Batteri SOH 92,5% enligt test")
	if result.SOHPercent == nil || *result.SOHPercent != 92.5 {
		t.Fatalf("soh_percent = %v", result.SOHPercent)
	}
	if !result.BatteryTested {
		t.Fatal("expected battery tested")
	}
	if result.SOHRawMatch == nil {
		t.Fatal("expected raw match")
	}
}

func TestExtractSOHPercentSuffix(t *testing.T) {
	result := soh.ExtractFromText("92% batterihälsa")
	if result.SOHPercent == nil || *result.SOHPercent != 92.0 {
		t.Fatalf("soh_percent = %v", result.SOHPercent)
	}
	if !result.BatteryTested {
		t.Fatal("expected battery tested")
	}
}

func TestExtractSOHBatteryTestedOnly(t *testing.T) {
	result := soh.ExtractFromText("Bilen är batteritestad hos Aviloo")
	if result.SOHPercent != nil {
		t.Fatalf("soh_percent = %v", result.SOHPercent)
	}
	if !result.BatteryTested {
		t.Fatal("expected battery tested")
	}
}

func TestExtractSOHEmpty(t *testing.T) {
	result := soh.ExtractFromText("")
	if result.SOHPercent != nil || result.BatteryTested {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplySOHPreservesPercentOnBatteryOnlyEnrich(t *testing.T) {
	listing := schema.CarListing{Source: "blocket", ID: "1", Title: "EV", Raw: map[string]any{}}
	soh.Apply(&listing, "blocket_search", "SOH 88%")
	soh.Apply(&listing, "blocket_detail", "Batteritestad")
	if listing.SOHPercent == nil || *listing.SOHPercent != 88.0 {
		t.Fatalf("soh_percent = %v", listing.SOHPercent)
	}
	if !listing.BatteryTested {
		t.Fatal("expected battery tested")
	}
	if listing.SOHSource == nil || *listing.SOHSource != "blocket_search" {
		t.Fatalf("soh_source = %v", listing.SOHSource)
	}
}
