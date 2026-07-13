package schema_test

import (
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
)

func TestNormalizeRegistrationNumber(t *testing.T) {
	if got := schema.NormalizeRegistrationNumber("rmc 648"); got != "RMC648" {
		t.Fatalf("got %q", got)
	}
	if got := schema.NormalizeRegistrationNumber("abc-12a"); got != "ABC12A" {
		t.Fatalf("got %q", got)
	}
}

func TestRegistrationMatches(t *testing.T) {
	reg := "RMC648"
	if !schema.RegistrationMatches(&reg, "rmc648") {
		t.Fatal("expected match")
	}
	if schema.RegistrationMatches(nil, "RMC648") {
		t.Fatal("expected no match for nil registration")
	}
}
