package wayke

import (
	"context"
	"net/url"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
)

// GetByLicensePlate searches Wayke for a vehicle with the given registration number.
func (c *Client) GetByLicensePlate(ctx context.Context, plate string) (*schema.CarListing, error) {
	plate = schema.NormalizeRegistrationNumber(plate)
	if plate == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("hits", "20")
	params.Set("registrationNumber", plate)
	html, err := c.fetchHTML(ctx, "/sok", params)
	if err != nil {
		return nil, err
	}
	for _, item := range extractDocuments(html) {
		listing := parseScrapedVehicle(item)
		if schema.RegistrationMatches(listing.RegistrationNumber, plate) {
			if detail, err := c.getVehicleScrape(ctx, listing.ID); err == nil && detail != nil {
				return detail, nil
			}
			return &listing, nil
		}
	}
	q := plate
	results, err := c.searchScrape(ctx, &q, 40, 1)
	if err != nil {
		return nil, err
	}
	for i := range results {
		if schema.RegistrationMatches(results[i].RegistrationNumber, plate) {
			if detail, err := c.getVehicleScrape(ctx, results[i].ID); err == nil && detail != nil {
				return detail, nil
			}
			return &results[i], nil
		}
	}
	return nil, nil
}
