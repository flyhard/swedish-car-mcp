package urls

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	blocketItem     = regexp.MustCompile(`(?i)/mobility/item/(?P<id>\d+)`)
	waykeVehicle    = regexp.MustCompile(`(?i)/(?:vehicle|fordon|bilar)/(?P<id>[^/?#]+)`)
	kvdObject       = regexp.MustCompile(`(?i)/(?:objekt|vehicle|bil|auktion)/(?P<id>\d+)`)
	traderaItem     = regexp.MustCompile(`(?i)/item/(?P<id>\d+)`)
	riddermarkCar   = regexp.MustCompile(`(?i)/kopa-bil/(?P<make>[^/]+)/(?P<id>[^/?#]+)`)
	carlaCar        = regexp.MustCompile(`(?i)/bil/(?P<id>[^/?#]+)`)
)

// ParseListingURL extracts source and listing id from a public URL.
func ParseListingURL(rawURL string) (source, id string, ok bool) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return "", "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
	path := parsed.Path

	if strings.HasSuffix(host, "blocket.se") {
		if m := blocketItem.FindStringSubmatch(path); len(m) >= 2 {
			return "blocket", m[1], true
		}
	}
	if strings.HasSuffix(host, "wayke.se") {
		if m := waykeVehicle.FindStringSubmatch(path); len(m) >= 2 {
			return "wayke", m[1], true
		}
	}
	if strings.HasSuffix(host, "kvd.se") {
		if m := kvdObject.FindStringSubmatch(path); len(m) >= 2 {
			return "kvd", m[1], true
		}
	}
	if strings.HasSuffix(host, "tradera.com") || strings.HasSuffix(host, "tradera.se") {
		if m := traderaItem.FindStringSubmatch(path); len(m) >= 2 {
			return "tradera", m[1], true
		}
	}
	if strings.HasSuffix(host, "riddermarkbil.se") {
		if m := riddermarkCar.FindStringSubmatch(path); len(m) >= 3 {
			return "riddermark", strings.ToLower(m[1]) + "/" + strings.ToLower(m[2]), true
		}
	}
	if strings.HasSuffix(host, "carla.se") {
		if m := carlaCar.FindStringSubmatch(path); len(m) >= 2 {
			return "carla", m[1], true
		}
	}
	return "", "", false
}
