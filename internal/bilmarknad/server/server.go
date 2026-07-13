package server

import (
	"context"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/search"
	"github.com/flyhard/swedish-car-mcp/internal/mcpjson"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.2.0"

var defaultService = &search.Service{}

type searchCarsInput struct {
	Query            *string  `json:"query"`
	Q                *string  `json:"q"`
	Make             *string  `json:"make"`
	Model            *string  `json:"model"`
	LicensePlate     *string  `json:"license_plate"`
	RegistrationNo   *string  `json:"registration_number"`
	RegNo            *string  `json:"regno"`
	RegNr            *string  `json:"regnr"`
	PriceMin         *int     `json:"price_min"`
	PriceMax         *int     `json:"price_max"`
	YearMin          *int     `json:"year_min"`
	YearMax          *int     `json:"year_max"`
	MileageMaxKM     *int     `json:"mileage_max_km"`
	FuelTypes        []string `json:"fuel_types"`
	Transmission     *string  `json:"transmission"`
	Sources          []string `json:"sources"`
	Limit            *int     `json:"limit"`
	Page             *int     `json:"page"`
	Sort             *string  `json:"sort"`
	UseBlocketProxy  *bool    `json:"use_blocket_proxy"`
}

type getListingInput struct {
	Source    *string `json:"source"`
	ID        *string `json:"id"`
	ListingID *string `json:"listing_id"`
	URL       *string `json:"url"`
}

func textQuery(in searchCarsInput) *string {
	if in.Query != nil {
		return in.Query
	}
	return in.Q
}

func licensePlate(in searchCarsInput) *string {
	for _, p := range []*string{in.LicensePlate, in.RegistrationNo, in.RegNo, in.RegNr} {
		if p != nil && strings.TrimSpace(*p) != "" {
			return p
		}
	}
	return nil
}

func searchCars(ctx context.Context, _ *mcp.CallToolRequest, in searchCarsInput) (*mcp.CallToolResult, struct{}, error) {
	useProxy := in.UseBlocketProxy != nil && *in.UseBlocketProxy
	svc := defaultService
	if useProxy {
		svc = &search.Service{UseBlocketProxy: true}
		defer svc.Close()
	}
	limit := 20
	if in.Limit != nil {
		limit = *in.Limit
	}
	page := 1
	if in.Page != nil {
		page = *in.Page
	}
	results, err := svc.SearchCars(ctx, search.SearchOptions{
		Query: textQuery(in), Make: in.Make, Model: in.Model, LicensePlate: licensePlate(in),
		PriceMin: in.PriceMin, PriceMax: in.PriceMax,
		YearMin: in.YearMin, YearMax: in.YearMax,
		MileageMaxKM: in.MileageMaxKM, FuelTypes: in.FuelTypes,
		Transmission: in.Transmission, Sources: in.Sources,
		Limit: limit, Page: page, Sort: in.Sort,
	})
	if err != nil {
		return mcpjson.ErrorResult(err.Error())
	}
	return mcpjson.TextResult(map[string]any{
		"count":    len(results),
		"listings": results,
	})
}

func getListing(ctx context.Context, _ *mcp.CallToolRequest, in getListingInput) (*mcp.CallToolResult, struct{}, error) {
	lid := in.ID
	if lid == nil {
		lid = in.ListingID
	}
	item := defaultService.GetListing(ctx, in.Source, lid, in.URL)
	if item != nil {
		return mcpjson.TextResult(item)
	}
	return mcpjson.TextResult(map[string]any{
		"found":  false,
		"source": in.Source,
		"id":     lid,
		"url":    in.URL,
	})
}

func listSources(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, struct{}, error) {
	return mcpjson.TextResult(defaultService.ListSources())
}

// Run starts the bilmarknad MCP server on stdio.
func Run(ctx context.Context) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "bilmarknad", Version: version}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "search_cars", Description: "Search used cars across Swedish marketplaces. Use license_plate (or registration_number/regno) for direct lookup by registration number."}, searchCars)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_listing", Description: "Fetch one listing by source+id, license plate, or public listing URL."}, getListing)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_sources", Description: "List supported sources and related environment variables."}, listSources)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
