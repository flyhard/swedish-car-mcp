package search

import (
	"context"
	"sync"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/presets"
)

// ListingRef identifies one listing for batch fetch.
type ListingRef struct {
	Source         *string `json:"source,omitempty"`
	ID             *string `json:"id,omitempty"`
	ListingID      *string `json:"listing_id,omitempty"`
	URL            *string `json:"url,omitempty"`
	LicensePlate   *string `json:"license_plate,omitempty"`
	RegistrationNo *string `json:"registration_number,omitempty"`
	RegNo          *string `json:"regno,omitempty"`
	RegNr          *string `json:"regnr,omitempty"`
}

// BatchGetListings fetches multiple listings in parallel.
func (s *Service) BatchGetListings(ctx context.Context, refs []ListingRef) []map[string]any {
	out := make([]map[string]any, len(refs))
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref ListingRef) {
			defer wg.Done()
			id := ref.ID
			if id == nil {
				id = ref.ListingID
			}
			plate := firstNonEmpty(ref.LicensePlate, ref.RegistrationNo, ref.RegNo, ref.RegNr)
			if plate != nil && (id == nil || *id == "") {
				id = plate
			}
			item := s.GetListing(ctx, ref.Source, id, ref.URL)
			if item != nil {
				out[i] = item
				return
			}
			out[i] = map[string]any{
				"found": false,
				"source": ref.Source,
				"id":     id,
				"url":    ref.URL,
			}
		}(i, ref)
	}
	wg.Wait()
	return out
}

// PresetScanResult is the outcome of one preset query.
type PresetScanResult struct {
	Preset   presets.ScanPreset `json:"preset"`
	Count    int                `json:"count"`
	Listings []map[string]any   `json:"listings"`
	Error    string             `json:"error,omitempty"`
}

// RunPresetScans executes daily-update preset queries.
func (s *Service) RunPresetScans(ctx context.Context, names []string, limit *int, useBlocketProxy bool) ([]PresetScanResult, error) {
	selected := presets.ByName(names)
	perScan := 20
	if limit != nil && *limit > 0 {
		perScan = *limit
	}
	svc := s
	if useBlocketProxy {
		svc = &Service{UseBlocketProxy: true}
		defer svc.Close()
	}
	results := make([]PresetScanResult, len(selected))
	for i, preset := range selected {
		listings, err := svc.SearchCars(ctx, SearchOptions{
			Query:        &preset.Query,
			PriceMin:     preset.PriceMin,
			PriceMax:     preset.PriceMax,
			MileageMaxKM: preset.MileageMaxKM,
			FuelTypes:    preset.FuelTypes,
			Sources:      []string{"blocket"},
			Limit:        perScan,
			Page:         1,
		})
		result := PresetScanResult{Preset: preset, Count: len(listings), Listings: listings}
		if err != nil {
			result.Error = err.Error()
		}
		results[i] = result
	}
	return results, nil
}

func firstNonEmpty(values ...*string) *string {
	for _, v := range values {
		if v != nil && *v != "" {
			return v
		}
	}
	return nil
}
