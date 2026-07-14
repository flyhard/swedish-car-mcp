package presets

// ScanPreset is one daily-update Blocket query from scan-queries.md.
type ScanPreset struct {
	Name         string   `json:"name"`
	Query        string   `json:"query"`
	PriceMin     *int     `json:"price_min,omitempty"`
	PriceMax     *int     `json:"price_max,omitempty"`
	MileageMaxKM *int     `json:"mileage_max_km,omitempty"`
	FuelTypes    []string `json:"fuel_types,omitempty"`
	ReVerify     []string `json:"re_verify,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

func intPtr(v int) *int { return &v }

// DailyUpdate returns all presets for the elbil-familj-kop daily update skill.
func DailyUpdate() []ScanPreset {
	return []ScanPreset{
		{
			Name: "kia_niro_ev", Query: "Kia Niro EV",
			PriceMin: intPtr(220000), PriceMax: intPtr(360000),
			FuelTypes: []string{"electric"},
			ReVerify:  []string{"FWA72J", "DJA16M", "PPJ14D", "NDZ12C", "JJN99F", "TMP78U"},
			Notes:     "300k primary; filter >10000 mil in ranking not query",
		},
		{
			Name: "kia_niro_ev_stretch", Query: "Kia Niro EV",
			PriceMin: intPtr(310000), PriceMax: intPtr(360000),
			FuelTypes: []string{"electric"},
			Notes:     "Stretch band for NDZ12C-class listings",
		},
		{
			Name: "kia_e_niro", Query: "Kia e-Niro",
			PriceMin: intPtr(170000), PriceMax: intPtr(320000),
			FuelTypes: []string{"electric"},
			ReVerify:  []string{"FKS68G", "UXD07G", "SZN57F", "APT93K", "GNW89A", "GUN700"},
		},
		{
			Name: "vw_id4", Query: "ID.4",
			PriceMin: intPtr(199000), PriceMax: intPtr(310000),
			FuelTypes: []string{"electric"},
			ReVerify:  []string{"TFE37L", "EBK074", "SYM278", "NBJ534", "RKX54S"},
		},
		{
			Name: "vw_id3", Query: "ID.3",
			PriceMin: intPtr(199000), PriceMax: intPtr(320000),
			FuelTypes: []string{"electric"},
			Notes:     "Focus ≤220k ACC (GTT598) and 300k stretch (RFK46U, EAC34X)",
		},
		{
			Name: "skoda_enyaq", Query: "Enyaq",
			PriceMin: intPtr(220000), PriceMax: intPtr(320000),
			FuelTypes: []string{"electric"},
			ReVerify:  []string{"CML75F"},
		},
		{
			Name: "hyundai_kona_electric", Query: "Kona Electric",
			PriceMin: intPtr(170000), PriceMax: intPtr(220000),
			FuelTypes: []string{"electric"},
			ReVerify:  []string{"YRW736"},
		},
	}
}

// ByName returns presets filtered by name (empty = all).
func ByName(names []string) []ScanPreset {
	all := DailyUpdate()
	if len(names) == 0 {
		return all
	}
	want := map[string]struct{}{}
	for _, n := range names {
		want[n] = struct{}{}
	}
	var out []ScanPreset
	for _, p := range all {
		if _, ok := want[p.Name]; ok {
			out = append(out, p)
		}
	}
	return out
}
