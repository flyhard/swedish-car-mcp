package search

import (
	"context"
	"os"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/blocket"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/blocketproxy"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/carla"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/kvd"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/riddermark"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/tradera"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/urls"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/wayke"
)

// AllSources is the default marketplace source list.
var AllSources = []string{"blocket", "wayke", "kvd", "tradera", "riddermark", "carla"}

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
}

// SearchOptions mirrors the search_cars MCP tool parameters.
type SearchOptions struct {
	Query         *string
	Make          *string
	Model         *string
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

func matchesFilters(item schema.CarListing, make, model *string, priceMin, priceMax, yearMin, yearMax, mileageMax *int) bool {
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

	for _, source := range active {
		switch source {
		case "blocket":
			fuels := opts.FuelTypes
			if len(fuels) == 0 {
				fuels = []string{""}
			}
			for _, fuel := range fuels {
				bp := blocket.SearchParams{
					Q: opts.Query, Make: opts.Make, Model: opts.Model,
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
						return nil, err
					}
					collected = append(collected, items...)
				} else {
					items, err := s.blocketClientTyped().Search(ctx, bp)
					if err != nil {
						return nil, err
					}
					collected = append(collected, items...)
				}
			}
		case "wayke":
			q := joinQuery(opts.Query, opts.Make, opts.Model)
			items, err := s.waykeClient().Search(ctx, q, limit, page)
			if err != nil {
				return nil, err
			}
			collected = append(collected, items...)
		case "kvd":
			items, err := s.kvdClient().Search(ctx)
			if err != nil {
				if _, ok := err.(kvd.UnavailableError); ok {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "tradera":
			q := joinQuery(opts.Query, opts.Make, opts.Model)
			items, err := s.traderaClient().Search(ctx, q, limit, page, "Relevance")
			if err != nil {
				if _, ok := err.(tradera.UnavailableError); ok {
					continue
				}
				return nil, err
			}
			collected = append(collected, items...)
		case "riddermark":
			items, err := s.riddermarkClient().Search(ctx, opts.Query, opts.Make, opts.Model, opts.PriceMin, opts.PriceMax, opts.MileageMaxKM, limit, page)
			if err != nil {
				return nil, err
			}
			collected = append(collected, items...)
		case "carla":
			var fuel *string
			if len(opts.FuelTypes) > 0 {
				fuel = &opts.FuelTypes[0]
			}
			items, err := s.carlaClient().Search(ctx, opts.Query, opts.Make, opts.Model, fuel, limit, page)
			if err != nil {
				return nil, err
			}
			collected = append(collected, items...)
		}
	}

	filtered := make([]schema.CarListing, 0, len(collected))
	for _, item := range collected {
		if matchesFilters(item, opts.Make, opts.Model, opts.PriceMin, opts.PriceMax, opts.YearMin, opts.YearMax, opts.MileageMaxKM) {
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
			{"id": "wayke", "description": "Wayke vehicle search (REST with API key or public GraphQL)", "env": []string{"WAYKE_API_KEY"}},
			{"id": "kvd", "description": "KVD public API probe (returns empty until a stable endpoint exists)", "env": []string{}},
			{"id": "tradera", "description": "Tradera car auctions and buy-now listings (REST API v3, 100 calls/day)", "env": []string{"TRADERA_APP_ID", "TRADERA_APP_KEY", "TRADERA_CAR_CATEGORY_ID"}},
			{"id": "riddermark", "description": "Riddermark Bil used-car search via Next.js page data", "env": []string{}},
			{"id": "carla", "description": "Carla EV marketplace search via Next.js data API", "env": []string{}},
		},
		"env": listSourcesEnv(),
	}
}

func listSourcesEnv() map[string]any {
	return map[string]any{
		"WAYKE_API_KEY": map[string]any{
			"required": false, "description": "Optional Wayke REST API bearer token",
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
