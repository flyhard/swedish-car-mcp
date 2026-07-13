package ayvens

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/soh"
)

const (
	busSecurityGraphQL = "https://bus2.bus.no/BUSPlatform3/SecurityPortal/BUS.APIs.GraphQLServer.P3.SecurityPortal.Endpoint1"
	busInternalGraphQL = "https://bus2.bus.no/BUSPlatform3/BUStest.Server/BUS.APIs.GraphQLServer.P3.BUStest.InternalApp"
	busSalesReportApp  = 25
)

const applyTokenQuery = `query queryApplyToken($tokenId: String, $tokenSecret: String, $applicationId: Int!, $additionalInfo: String) {
  tokens: token_ApplyToken(tokenId: $tokenId, tokenSecret: $tokenSecret, applicationId: $applicationId, additionalInfo: $additionalInfo) {
    accessToken
    idToken
    refreshToken
  }
}`

const batteryMetaQuery = `query getBatteryTestMetaData($testId: Int!) {
  batteryTestMetaData: test_GetBatteryTestMetaData(testId: $testId) {
    batteryResult
    performedBatteryMethod
    performedBatteryNote
    isBatteryReadingExcludeFromReport
    hasAvilooDevices
  }
}`

const avilooScoreQuery = `query getAvilooBatteryScore($testId: Int!) {
  avilooBatteryScore: test_GetAvilooBatteryScore(testId: $testId)
}`

const avilooReportQuery = `query getAvilooReportForTest($testId: Int!) {
  avilooReport: test_GetAvilooReportForTest(testId: $testId) {
    externalFileId
    fileName
    sasUrl
  }
}`

type busLink struct {
	TokenID     string
	TokenSecret string
}

type busSession struct {
	AccessToken   string
	TestID        int
	SalesReportID int
}

type busBatteryData struct {
	BatteryResult   *float64
	AvilooScore     *float64
	AvilooReportURL *string
	Note            string
}

func parseBUSLink(reportURL string) (busLink, bool) {
	reportURL = strings.TrimSpace(reportURL)
	if reportURL == "" {
		return busLink{}, false
	}
	u, err := url.Parse(reportURL)
	if err != nil {
		return busLink{}, false
	}
	fragment := strings.TrimPrefix(u.Fragment, "/")
	if !strings.HasPrefix(fragment, "salesReportLink") {
		return busLink{}, false
	}
	qIdx := strings.Index(fragment, "?")
	if qIdx < 0 {
		return busLink{}, false
	}
	vals, err := url.ParseQuery(fragment[qIdx+1:])
	if err != nil {
		return busLink{}, false
	}
	tid := strings.TrimSpace(vals.Get("tid"))
	ts := strings.TrimSpace(vals.Get("ts"))
	if tid == "" || ts == "" {
		return busLink{}, false
	}
	return busLink{TokenID: tid, TokenSecret: ts}, true
}

func (c *Client) enrichSOHFromInspection(ctx context.Context, listing *schema.CarListing, reportURL string) {
	link, ok := parseBUSLink(reportURL)
	if !ok {
		return
	}
	session, err := c.busApplyToken(ctx, link)
	if err != nil {
		return
	}
	battery, err := c.busFetchBattery(ctx, session)
	if err != nil {
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
	if battery.AvilooReportURL != nil {
		listing.Raw["aviloo_report_url"] = *battery.AvilooReportURL
	}
	if pct := batteryBestSOH(battery); pct != nil {
		v := *pct
		listing.SOHPercent = &v
		src := "ayvens_bus"
		listing.SOHSource = &src
		match := fmt.Sprintf("%.0f%%", v)
		listing.SOHRawMatch = &match
		listing.BatteryTested = true
		return
	}
	fields := busSOHTextFields(battery)
	if len(fields) > 0 {
		soh.Apply(listing, "ayvens_bus", fields...)
	}
}

func batteryBestSOH(data busBatteryData) *float64 {
	if data.AvilooScore != nil && *data.AvilooScore > 0 && *data.AvilooScore <= 100 {
		return data.AvilooScore
	}
	if data.BatteryResult != nil && *data.BatteryResult > 0 && *data.BatteryResult <= 100 {
		return data.BatteryResult
	}
	return nil
}

func busSOHTextFields(data busBatteryData) []string {
	var out []string
	if data.Note != "" {
		out = append(out, data.Note)
	}
	if data.BatteryResult != nil {
		out = append(out, fmt.Sprintf("batteryResult %v", *data.BatteryResult))
	}
	if data.AvilooScore != nil {
		out = append(out, fmt.Sprintf("avilooBatteryScore %v", *data.AvilooScore))
	}
	return out
}

func (c *Client) busApplyToken(ctx context.Context, link busLink) (busSession, error) {
	var resp struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"accessToken"`
			} `json:"tokens"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.busGraphQL(ctx, busSecurityGraphQL, applyTokenQuery, map[string]any{
		"tokenId": link.TokenID, "tokenSecret": link.TokenSecret, "applicationId": busSalesReportApp,
	}, "", &resp); err != nil {
		return busSession{}, err
	}
	if len(resp.Errors) > 0 {
		return busSession{}, fmt.Errorf("bus apply token: %v", resp.Errors)
	}
	payload, err := decodeJWTClaims(resp.Data.Tokens.AccessToken)
	if err != nil {
		return busSession{}, err
	}
	inner, err := businessTokenFromClaims(payload)
	if err != nil {
		return busSession{}, err
	}
	return busSession{
		AccessToken:   resp.Data.Tokens.AccessToken,
		TestID:        intFromJSON(inner["testId"]),
		SalesReportID: intFromJSON(inner["salesReportId"]),
	}, nil
}

func (c *Client) busFetchBattery(ctx context.Context, session busSession) (busBatteryData, error) {
	if session.TestID <= 0 || session.AccessToken == "" {
		return busBatteryData{}, fmt.Errorf("bus: missing test id or token")
	}
	out := busBatteryData{}
	var metaResp struct {
		Data struct {
			Meta struct {
				BatteryResult                     *float64 `json:"batteryResult"`
				PerformedBatteryNote              *string  `json:"performedBatteryNote"`
				IsBatteryReadingExcludeFromReport bool     `json:"isBatteryReadingExcludeFromReport"`
			} `json:"batteryTestMetaData"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.busGraphQL(ctx, busInternalGraphQL, batteryMetaQuery, map[string]any{"testId": session.TestID}, session.AccessToken, &metaResp); err == nil && len(metaResp.Errors) == 0 {
		if metaResp.Data.Meta.IsBatteryReadingExcludeFromReport {
			return busBatteryData{}, fmt.Errorf("bus: battery reading excluded from report")
		}
		out.BatteryResult = metaResp.Data.Meta.BatteryResult
		if metaResp.Data.Meta.PerformedBatteryNote != nil {
			out.Note = strings.TrimSpace(*metaResp.Data.Meta.PerformedBatteryNote)
		}
	}

	var scoreResp struct {
		Data struct {
			Score *float64 `json:"avilooBatteryScore"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.busGraphQL(ctx, busInternalGraphQL, avilooScoreQuery, map[string]any{"testId": session.TestID}, session.AccessToken, &scoreResp); err == nil && len(scoreResp.Errors) == 0 {
		out.AvilooScore = scoreResp.Data.Score
	}

	var reportResp struct {
		Data struct {
			Report *struct {
				SasURL string `json:"sasUrl"`
			} `json:"avilooReport"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.busGraphQL(ctx, busInternalGraphQL, avilooReportQuery, map[string]any{"testId": session.TestID}, session.AccessToken, &reportResp); err == nil && len(reportResp.Errors) == 0 {
		if reportResp.Data.Report != nil && strings.TrimSpace(reportResp.Data.Report.SasURL) != "" {
			u := strings.TrimSpace(reportResp.Data.Report.SasURL)
			out.AvilooReportURL = &u
		}
	}

	if out.BatteryResult == nil && out.AvilooScore == nil && out.Note == "" && out.AvilooReportURL == nil {
		return busBatteryData{}, fmt.Errorf("bus: no battery data")
	}
	return out, nil
}

func (c *Client) busGraphQL(ctx context.Context, endpoint, query string, variables map[string]any, accessToken string, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "bus", httputil.DefaultRetryPolicy())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("bus graphql: %s: %s", resp.Status, string(raw))
	}
	return json.Unmarshal(raw, out)
}

func decodeJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("bus: invalid jwt")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	return claims, json.Unmarshal(payload, &claims)
}

func businessTokenFromClaims(claims map[string]any) (map[string]any, error) {
	bt, ok := claims["businessToken"].(string)
	if !ok || strings.TrimSpace(bt) == "" {
		return nil, fmt.Errorf("bus: missing businessToken")
	}
	var wrapper map[string]any
	if err := json.Unmarshal([]byte(bt), &wrapper); err != nil {
		return nil, err
	}
	val, ok := wrapper["value"].(string)
	if !ok || strings.TrimSpace(val) == "" {
		return nil, fmt.Errorf("bus: missing business token value")
	}
	var inner map[string]any
	return inner, json.Unmarshal([]byte(val), &inner)
}

func intFromJSON(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
