package ayvens

import "testing"

func TestParseBUSLink(t *testing.T) {
	link, ok := parseBUSLink("https://bus2.bus.no/BUSPlatform3/BUStest.Client/#/salesReportLink?tid=abc&ts=secret%24token")
	if !ok || link.TokenID != "abc" || link.TokenSecret != "secret$token" {
		t.Fatalf("got %+v ok=%v", link, ok)
	}
	if _, ok := parseBUSLink("https://example.com/"); ok {
		t.Fatal("expected false for unrelated url")
	}
}

func TestBatteryBestSOH(t *testing.T) {
	score := 96.0
	result := 94.0
	if got := batteryBestSOH(busBatteryData{AvilooScore: &score, BatteryResult: &result}); got == nil || *got != 96 {
		t.Fatalf("got %v", got)
	}
	if got := batteryBestSOH(busBatteryData{BatteryResult: &result}); got == nil || *got != 94 {
		t.Fatalf("got %v", got)
	}
}
