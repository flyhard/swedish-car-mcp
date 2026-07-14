package bus_test

import (
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/bus"
)

func TestParseLink(t *testing.T) {
	link, ok := bus.ParseLink("https://bus2.bus.no/BUSPlatform3/BUStest.Client/#/salesReportLink?tid=abc&ts=secret%24token")
	if !ok || link.TokenID != "abc" || link.TokenSecret != "secret$token" {
		t.Fatalf("got %+v ok=%v", link, ok)
	}
	if _, ok := bus.ParseLink("https://example.com/"); ok {
		t.Fatal("expected false for unrelated url")
	}
}

func TestBestSOH(t *testing.T) {
	score := 96.0
	result := 94.0
	if got := bus.BestSOH(&bus.Report{AvilooBatteryScore: &score, BatteryResult: &result}); got == nil || *got != 96 {
		t.Fatalf("got %v", got)
	}
	if got := bus.BestSOH(&bus.Report{BatteryResult: &result}); got == nil || *got != 94 {
		t.Fatalf("got %v", got)
	}
}
