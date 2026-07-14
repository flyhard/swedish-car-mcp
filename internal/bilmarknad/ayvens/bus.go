package ayvens

import (
	"context"
	"fmt"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/bus"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

func (c *Client) enrichSOHFromInspection(ctx context.Context, listing *schema.CarListing, reportURL string) {
	if _, ok := bus.ParseLink(reportURL); !ok {
		return
	}
	busClient := bus.NewClient(c.httpClient)
	session, err := busClient.FetchReport(ctx, reportURL)
	if err != nil || session == nil {
		return
	}
	if listing.Raw == nil {
		listing.Raw = map[string]any{}
	}
	if session.TestID > 0 {
		listing.Raw["inspection_test_id"] = session.TestID
	}
	if session.SalesReportID > 0 {
		listing.Raw["inspection_sales_report_id"] = session.SalesReportID
	}
	if session.AvilooReportURL != nil {
		listing.Raw["aviloo_report_url"] = *session.AvilooReportURL
	}
	if pct := bus.BestSOH(session); pct != nil {
		v := *pct
		listing.SOHPercent = &v
		src := "ayvens_bus"
		listing.SOHSource = &src
		match := fmt.Sprintf("%.0f%%", v)
		listing.SOHRawMatch = &match
		listing.BatteryTested = true
		return
	}
	fields := busSOHTextFields(session)
	if len(fields) > 0 {
		soh.Apply(listing, "ayvens_bus", fields...)
	}
}

func busSOHTextFields(data *bus.Report) []string {
	var out []string
	if data.PerformedBatteryNote != "" {
		out = append(out, data.PerformedBatteryNote)
	}
	if data.BatteryResult != nil {
		out = append(out, fmt.Sprintf("batteryResult %v", *data.BatteryResult))
	}
	if data.AvilooBatteryScore != nil {
		out = append(out, fmt.Sprintf("avilooBatteryScore %v", *data.AvilooBatteryScore))
	}
	return out
}
