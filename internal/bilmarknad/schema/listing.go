package schema

// CarListing is the normalized listing shape returned by MCP tools.
type CarListing struct {
	Source             string         `json:"source"`
	ID                 string         `json:"id"`
	Title              string         `json:"title"`
	Make               *string        `json:"make"`
	Model              *string        `json:"model"`
	Year               *int           `json:"year"`
	MileageKM          *int           `json:"mileage_km"`
	PriceSEK           *int           `json:"price_sek"`
	Fuel               *string        `json:"fuel"`
	Transmission       *string        `json:"transmission"`
	Location           *string        `json:"location"`
	DealerName         *string        `json:"dealer_name"`
	URL                *string        `json:"url"`
	ImageURL           *string        `json:"image_url"`
	PublishedAt        *string        `json:"published_at"`
	RegistrationNumber *string        `json:"registration_number"`
	SOHPercent         *float64       `json:"soh_percent"`
	BatteryTested      bool           `json:"battery_tested"`
	SOHSource          *string        `json:"soh_source"`
	SOHRawMatch        *string        `json:"soh_raw_match"`
	Raw                map[string]any `json:"raw"`
}

// ToMap returns a JSON-serializable map matching Python asdict output.
func (c CarListing) ToMap() map[string]any {
	return map[string]any{
		"source":              c.Source,
		"id":                  c.ID,
		"title":               c.Title,
		"make":                c.Make,
		"model":               c.Model,
		"year":                c.Year,
		"mileage_km":          c.MileageKM,
		"price_sek":           c.PriceSEK,
		"fuel":                c.Fuel,
		"transmission":        c.Transmission,
		"location":            c.Location,
		"dealer_name":         c.DealerName,
		"url":                 c.URL,
		"image_url":           c.ImageURL,
		"published_at":        c.PublishedAt,
		"registration_number": c.RegistrationNumber,
		"soh_percent":         c.SOHPercent,
		"battery_tested":      c.BatteryTested,
		"soh_source":          c.SOHSource,
		"soh_raw_match":       c.SOHRawMatch,
		"raw":                 c.Raw,
	}
}

func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func IntPtr(v int) *int { return &v }

func FloatPtr(v float64) *float64 { return &v }
