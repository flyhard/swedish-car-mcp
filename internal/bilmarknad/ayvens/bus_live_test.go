package ayvens_test

import (
	"context"
	"os"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/ayvens"
)

func TestAyvensLiveBUSIntegration(t *testing.T) {
	if os.Getenv("AYVENS_LIVE") != "1" {
		t.Skip("set AYVENS_LIVE=1 to run")
	}
	client := ayvens.NewClient(nil)
	defer client.Close()
	listing, err := client.GetListing(context.Background(), "koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq")
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil {
		t.Fatal("nil listing")
	}
	if listing.SOHPercent == nil {
		t.Fatalf("missing soh on live listing: %+v", listing)
	}
	t.Logf("SOH=%v source=%v batteryTested=%v", *listing.SOHPercent, listing.SOHSource, listing.BatteryTested)
}
