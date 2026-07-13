package urls_test

import (
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
)

func TestURLBlocket(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.blocket.se/mobility/item/12345")
	if !ok || s != "blocket" || id != "12345" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

func TestURLTradera(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.tradera.com/item/123456")
	if !ok || s != "tradera" || id != "123456" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
	s, id, ok = urls.ParseListingURL("https://www.tradera.se/item/99")
	if !ok || s != "tradera" || id != "99" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

func TestURLRiddermark(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.riddermarkbil.se/kopa-bil/kia/ABC123/")
	if !ok || s != "riddermark" || id != "kia/abc123" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

func TestURLCarla(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://www.carla.se/bil/bmw-x1-2021-d4jbgei9io6g00fpicvg")
	if !ok || s != "carla" || id != "bmw-x1-2021-d4jbgei9io6g00fpicvg" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}

func TestURLAyvens(t *testing.T) {
	s, id, ok := urls.ParseListingURL("https://usedcars.ayvens.com/sv-se/koda-enyaq/tmbjc7ny2pf043314-skoda-enyaq.html")
	if !ok || s != "ayvens" || id != "tmbjc7ny2pf043314-skoda-enyaq" {
		t.Fatalf("got %q %q %v", s, id, ok)
	}
}
