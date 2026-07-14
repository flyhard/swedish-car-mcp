package presets_test

import (
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/presets"
)

func TestDailyUpdatePresets(t *testing.T) {
	all := presets.DailyUpdate()
	if len(all) < 7 {
		t.Fatalf("expected at least 7 presets, got %d", len(all))
	}
	subset := presets.ByName([]string{"vw_id4", "missing"})
	if len(subset) != 1 || subset[0].Name != "vw_id4" {
		t.Fatalf("subset = %+v", subset)
	}
}
