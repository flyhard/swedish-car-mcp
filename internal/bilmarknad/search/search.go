package search

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/ayvens"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/blocket"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/blocketproxy"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/carla"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/kvd"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/riddermark"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/tradera"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/wayke"
)

// AllSources is the default marketplace source list.
var AllSources = []string{"blocket", "wayke", "kvd", "tradera", "riddermark", "carla", "ayvens"}

// Service aggregates marketplace clients.
type Service struct {
	UseBlocketProxy bool
	blocket         *blocket.Client
	blocketProxy    *blocketproxy.Client
	wayke           *wayke.Client
	kvd             *kvd.Client
	tradera         *tradera.Client
	riddermark      *riddermark.Client
	carla           *carla.Client
	ayvens          *ayvens.Client
}

// SearchOptions mirrors the search_cars MCP tool parameters.
type SearchOptions struct {
	Query         *string
	Make          *string
	Model         *string
	LicensePlate  *string
	PriceMin      *int
	PriceMax      *int
	YearMin       *int
	YearMax       *int
	MileageMaxKM  *int
	FuelTypes     []string
	Transmission  *string
	Sources       []string
	Limit         int
	Page          int
	Sort          *string
}

// NormalizeSources returns validated source ids.
func NormalizeSources(sources []string) []string {
	if len(sources) == 0 {
		return append([]string(nil), AllSources...)
	}
	allowed := map[string]struct{}{}
	for _, s := range AllSources {
		allowed[s] = struct{}{}
	}
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range sources {
		key := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := allowed[key]; !ok {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		return append([]string(nil), AllSources...)
	}
	return out
}

func listingKey(item schema.CarListing) string {
	return item.Source + ":" + item.ID
}

func dedupeListings(items []schema.CarListing) []schema.CarListing {
	seen := map[string]struct{}{}
	out := make([]schema.CarListing, 0, len(items))
	for _, item := range items {
		key := listingKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func matchesFilters(item schema.CarListing, make, model *string, priceMin, priceMax, yearMin, yearMax, mileageMax *int, licensePlate string) bool {
	if licensePlate != "" && !schema.RegistrationMatches(item.RegistrationNumber, licensePlate) {
		if !mentionsLicensePlate(item, licensePlate) {
			return false
		}
	}
	if make != nil && *make != "" && item.Make != nil && !strings.Contains(strings.ToLower(*item.Make), strings.ToLower(strings.TrimSpace(*make))) {
		return false
	}
	if model != nil && *model != "" && item.Model != nil && !strings.Contains(strings.ToLower(*item.Model), strings.ToLower(strings.TrimSpace(*model))) {
		return false
	}
	if priceMin != nil && item.PriceSEK != nil && *item.PriceSEK < *priceMin {
		return false
	}
	if priceMax != nil && item.PriceSEK != nil && *item.PriceSEK > *priceMax {
		return false
	}
	if yearMin != nil && item.Year != nil && *item.Year < *yearMin {
		return false
	}
	if yearMax != nil && item.Year != nil && *item.Year > *yearMax {
		return false
	}
	if mileageMax != nil && item.MileageKM != nil && *item.MileageKM > *mileageMax {
		return false
	}
	return true
}

func (s *Service) blocketClient() any {
	if s.UseBlocketProxy {
		if s.blocketProxy == nil {
			s.blocketProxy = blocketproxy.NewClient(nil, "")
		}
		return s.blocketProxy
	}
	if s.blocket == nil {
		s.blocket = blocket.NewClient(nil)
	}
	return s.blocket
}

func (s *Service) waykeClient() *wayke.Client {
	if s.wayke == nil {
		s.wayke = wayke.NewClient(nil, "")
	}
	return s.wayke
}

func (s *Service) kvdClient() *kvd.Client {
	if s.kvd == nil {
		s.kvd = kvd.NewClient(nil)
	}
	return s.kvd
}

func (s *Service) traderaClient() *tradera.Client {
	if s.tradera == nil {
		s.tradera = tradera.NewClient(nil, "", "", 0)
	}
	return s.tradera
}

func (s *Service) riddermarkClient() *riddermark.Client {
	if s.riddermark == nil {
		s.riddermark = riddermark.NewClient(nil)
	}
	return s.riddermark
}

func (s *Service) ayvensClient() *ayvens.Client {
	if s.ayvens == nil {
		s.ayvens = ayvens.NewClient(nil)
	}
	return s.ayvens
}

func (s *Service) carlaClient() *carla.Client {
	if s.carla == nil {
		s.carla = carla.NewClient(nil)
	}
	return s.carla
}

// Close releases owned HTTP clients.
func (s *Service) Close() {
	if s.blocket != nil {
		s.blocket.Close()
	}
	if s.blocketProxy != nil {
		s.blocketProxy.Close()
	}
	if s.wayke != nil {
		s.wayke.Close()
	}
	if s.kvd != nil {
		s.kvd.Close()
	}
	if s.tradera != nil {
		s.tradera.Close()
	}
	if s.riddermark != nil {
		s.riddermark.Close()
	}
	if s.carla != nil {
		s.carla.Close()
	}
	if s.ayvens != nil {
		s.ayvens.Close()
	}
}

// SearchCars queries selected sources and returns listing maps.
func (s *Service) SearchCars(ctx context.Context, opts SearchOptions) ([]map[string]any, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	page := opts.Page
	if page == 0 {
		page = 1
	}
	active := NormalizeSources(opts.Sources)
	var collected []schema.CarListing
	licensePlate := ""
	if opts.LicensePlate != nil {
		licensePlate = schema.NormalizeRegistrationNumber(*opts.LicensePlate)
	}
	searchQuery := primaryQuery(opts)

	for _, source := range active {
		if licensePlate != "" {
			if item := s.lookupByLicensePlate(ctx, source, licensePlate); item != nil {
				collected = append(collected, *item)
				continue
			}
		}
		switch source {
		case "blocket":
			fuels := opts.FuelTypes
			if len(fuels) == 0 {
				fuels = []string{""}
			}
			for _, fuel := range fuels {
				bp := blocket.SearchParams{
					Q: searchQuery, Make: opts.Make, Model: opts.Model,
					PriceFrom: opts.PriceMin, PriceTo: opts.PriceMax,
					YearFrom: opts.YearMin, YearTo: opts.YearMax,
					MileageToKM: opts.MileageMaxKM, Transmission: opts.Transmission,
					Sort: opts.Sort, Rows: limit, Page: page,
				}
				if fuel != "" {
					bp.Fuel = fuel
				}
				if s.UseBlocketProxy {
					params := blocket.BuildParams(bp)
					items, err := s.blocketProxyClient().Search(ctx, params)
					if err != nil {
						if isSkippableSourceErr(err) {
							break
						}
						return nil, err
					}
					collected = append(collected, items...)
				} else {
					items, err := s.blocketClientTyped().Search(ctx, bp)
					if err != nil {
						if isSkippableSourceErr(err) {
							break
						}
						return nil, err
					}
					collected = append(collected, items...)
				}
			}
		case "wayke":
			items, err := s.waykeClient().Search(ctx, searchQuery, limit, page)
			if err != nil {
				if isSkippableSourceErr(err) {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "kvd":
			items, err := s.kvdClient().Search(ctx)
			if err != nil {
				if isSkippableSourceErr(err) {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "tradera":
			items, err := s.traderaClient().Search(ctx, searchQuery, limit, page, "Relevance")
			if err != nil {
				if isSkippableSourceErr(err) {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "riddermark":
			items, err := s.riddermarkClient().Search(ctx, searchQuery, opts.Make, opts.Model, opts.PriceMin, opts.PriceMax, opts.MileageMaxKM, limit, page)
			if err != nil {
				if isSkippableSourceErr(err) {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "carla":
			var fuel *string
			if len(opts.FuelTypes) > 0 {
				fuel = &opts.FuelTypes[0]
			}
			items, err := s.carlaClient().Search(ctx, searchQuery, opts.Make, opts.Model, fuel, limit, page)
			if err != nil {
				if isSkippableSourceErr(err) {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "ayvens":
			items, err := s.ayvensClient().Search(ctx, searchQuery, opts.Make, opts.Model, opts.PriceMin, opts.PriceMax, opts.MileageMaxKM, limit, page)
			if err != nil {
				if isSkippableSourceErr(err) {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		}
	}

	filtered := make([]schema.CarListing, 0, len(collected))
	for _, item := range collected {
		if matchesFilters(item, opts.Make, opts.Model, opts.PriceMin, opts.PriceMax, opts.YearMin, opts.YearMax, opts.MileageMaxKM, licensePlate) {
			filtered = append(filtered, item)
		}
	}
	deduped := dedupeListings(filtered)
	if len(deduped) > limit {
		deduped = deduped[:limit]
	}
	out := make([]map[string]any, len(deduped))
	for i, item := range deduped {
		out[i] = item.ToMap()
	}
	return out, nil
}

func isSkippableSourceErr(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(kvd.UnavailableError); ok {
		return true
	}
	if _, ok := err.(tradera.UnavailableError); ok {
		return true
	}
	return httputil.IsRateLimited(err)
}

func (s *Service) blocketClientTyped() *blocket.Client {
	if s.blocket == nil {
		s.blocket = blocket.NewClient(nil)
	}
	return s.blocket
}

func (s *Service) blocketProxyClient() *blocketproxy.Client {
	if s.blocketProxy == nil {
		s.blocketProxy = blocketproxy.NewClient(nil, "")
	}
	return s.blocketProxy
}

func joinQuery(parts ...*string) *string {
	var values []string
	for _, p := range parts {
		if p != nil && strings.TrimSpace(*p) != "" {
			values = append(values, strings.TrimSpace(*p))
		}
	}
	if len(values) == 0 {
		return nil
	}
	joined := strings.Join(values, " ")
	return &joined
}

func primaryQuery(opts SearchOptions) *string {
	if opts.LicensePlate != nil {
		if plate := schema.NormalizeRegistrationNumber(*opts.LicensePlate); plate != "" {
			return &plate
		}
	}
	return joinQuery(opts.Query, opts.Make, opts.Model)
}

func (s *Service) lookupByLicensePlate(ctx context.Context, source, plate string) *schema.CarListing {
	plate = schema.NormalizeRegistrationNumber(plate)
	if plate == "" {
		return nil
	}
	var (
		item *schema.CarListing
		err  error
	)
	switch source {
	case "blocket":
		item, err = s.blocketClientTyped().GetListing(ctx, plate)
	case "wayke":
		item, err = s.waykeClient().GetByLicensePlate(ctx, plate)
	case "riddermark":
		item, err = s.riddermarkClient().GetListing(ctx, plate)
	case "carla":
		item, err = s.carlaClient().GetListing(ctx, plate)
	case "ayvens":
		item, err = s.ayvensClient().GetListing(ctx, plate)
	case "tradera":
		item = s.lookupTraderaByLicensePlate(ctx, plate)
	case "kvd":
		return nil
	default:
		return nil
	}
	if err != nil || item == nil {
		return nil
	}
	if schema.RegistrationMatches(item.RegistrationNumber, plate) {
		return item
	}
	if mentionsLicensePlate(*item, plate) {
		return item
	}
	return nil
}

func (s *Service) lookupTraderaByLicensePlate(ctx context.Context, plate string) *schema.CarListing {
	results, err := s.traderaClient().Search(ctx, &plate, 20, 1, "Relevance")
	if err != nil {
		return nil
	}
	for i := range results {
		if mentionsLicensePlate(results[i], plate) {
			if detail, err := s.traderaClient().GetListing(ctx, results[i].ID); err == nil && detail != nil {
				return detail
			}
			return &results[i]
		}
	}
	return nil
}

func mentionsLicensePlate(item schema.CarListing, plate string) bool {
	plate = schema.NormalizeRegistrationNumber(plate)
	if plate == "" {
		return true
	}
	fields := []string{item.Title}
	if item.RegistrationNumber != nil {
		fields = append(fields, *item.RegistrationNumber)
	}
	for _, key := range []string{"short_description", "long_description", "description", "shortDescription"} {
		if v, ok := item.Raw[key]; ok {
			fields = append(fields, fmt.Sprint(v))
		}
	}
	needle := strings.ToUpper(plate)
	for _, field := range fields {
		if strings.Contains(schema.NormalizeRegistrationNumber(field), needle) {
			return true
		}
	}
	return false
}

// GetListing fetches a single listing by source/id or URL.
func (s *Service) GetListing(ctx context.Context, source, listingID, rawURL *string) map[string]any {
	src := ""
	id := ""
	if rawURL != nil && *rawURL != "" {
		if s, i, ok := urls.ParseListingURL(*rawURL); ok {
			src, id = s, i
		}
	}
	if source != nil && *source != "" {
		src = strings.ToLower(strings.TrimSpace(*source))
	}
	if listingID != nil && *listingID != "" {
		id = *listingID
	}
	if src == "" && id != "" {
		plate := schema.NormalizeRegistrationNumber(id)
		if plate != "" && len(plate) <= 7 {
			for _, source := range AllSources {
				if item := s.lookupByLicensePlate(ctx, source, plate); item != nil {
					return item.ToMap()
				}
			}
		}
	}
	if src == "" || id == "" {
		return nil
	}

	var item *schema.CarListing
	switch src {
	case "blocket":
		item, _ = s.blocketClientTyped().GetListing(ctx, id)
	case "wayke":
		item, _ = s.waykeClient().GetVehicle(ctx, id)
	case "kvd":
		items, err := s.kvdClient().Search(ctx)
		if err != nil {
			return nil
		}
		for i := range items {
			if items[i].ID == id {
				m := items[i].ToMap()
				return m
			}
		}
		return nil
	case "tradera":
		item, _ = s.traderaClient().GetListing(ctx, id)
	case "riddermark":
		item, _ = s.riddermarkClient().GetListing(ctx, id)
	case "carla":
		item, _ = s.carlaClient().GetListing(ctx, id)
	case "ayvens":
		item, _ = s.ayvensClient().GetListing(ctx, id)
	default:
		return nil
	}
	if item == nil {
		return nil
	}
	m := item.ToMap()
	return m
}

// ListSources returns supported sources and env metadata.
func (s *Service) ListSources() map[string]any {
	return map[string]any{
		"sources": []map[string]any{
			{"id": "blocket", "description": "Blocket mobility used-car search API", "env": []string{"BLOCKET_PROXY_URL"}},
			{"id": "wayke", "description": "Wayke vehicle search (website scrape; optional REST API with dealer key)", "env": []string{"WAYKE_API_KEY"}},
			{"id": "kvd", "description": "KVD public API probe (returns empty until a stable endpoint exists)", "env": []string{}},
			{"id": "tradera", "description": "Tradera car auctions and buy-now listings (REST API v3, 100 calls/day)", "env": []string{"TRADERA_APP_ID", "TRADERA_APP_KEY", "TRADERA_CAR_CATEGORY_ID"}},
			{"id": "riddermark", "description": "Riddermark Bil used-car search via Next.js page data", "env": []string{}},
			{"id": "carla", "description": "Carla EV marketplace search via Next.js data API", "env": []string{}},
			{"id": "ayvens", "description": "Ayvens used-car search (usedcars.ayvens.com) with DEKRA inspection report links", "env": []string{}},
		},
		"env": listSourcesEnv(),
	}
}

func listSourcesEnv() map[string]any {
	return map[string]any{
		"WAYKE_API_KEY": map[string]any{
			"required": false, "description": "Optional Wayke REST API bearer token (dealers only; scrape is used without it)",
			"set": os.Getenv("WAYKE_API_KEY") != "",
		},
		"BLOCKET_PROXY_URL": map[string]any{
			"required": false, "description": "Optional Blocket search proxy base URL",
			"set": os.Getenv("BLOCKET_PROXY_URL") != "",
		},
		"TRADERA_APP_ID": map[string]any{
			"required": false, "description": "Tradera developer app ID (defaults to shared dev credentials)",
			"set": os.Getenv("TRADERA_APP_ID") != "",
		},
		"TRADERA_APP_KEY": map[string]any{
			"required": false, "description": "Tradera developer app key (defaults to shared dev credentials)",
			"set": os.Getenv("TRADERA_APP_KEY") != "",
		},
		"TRADERA_CAR_CATEGORY_ID": map[string]any{
			"required": false, "description": "Tradera category ID for car searches (default 10 = Bilar)",
			"set": os.Getenv("TRADERA_CAR_CATEGORY_ID") != "",
		},
	}
}
