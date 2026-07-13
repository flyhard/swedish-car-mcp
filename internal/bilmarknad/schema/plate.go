package schema

import "strings"

// NormalizeRegistrationNumber uppercases a Swedish plate and strips spaces/dashes.
func NormalizeRegistrationNumber(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

// RegistrationMatches reports whether listing reg number equals the normalized plate.
func RegistrationMatches(registration *string, plate string) bool {
	plate = NormalizeRegistrationNumber(plate)
	if plate == "" {
		return true
	}
	if registration == nil {
		return false
	}
	return NormalizeRegistrationNumber(*registration) == plate
}
