package bus

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
)

const (
	SecurityGraphQL = "https://bus2.bus.no/BUSPlatform3/SecurityPortal/BUS.APIs.GraphQLServer.P3.SecurityPortal.Endpoint1"
	InternalGraphQL = "https://bus2.bus.no/BUSPlatform3/BUStest.Server/BUS.APIs.GraphQLServer.P3.BUStest.InternalApp"
	SalesReportApp  = 25
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

const salesReportQuery = `query GetSalesReportDetails($salesReportId: Int!) {
  salesReport: salesReport_GetSalesReport(salesReportId: $salesReportId) {
    testId
    mileage
    completedDate
    hasBatteryReading
    hasCreatedFromTestBatteryScore
  }
}`

// Link holds BUS salesReportLink token parameters.
type Link struct {
	TokenID     string
	TokenSecret string
}

// Report is the battery/DEKRA data resolved from a BUS sales report link.
type Report struct {
	InspectionURL      string   `json:"inspection_url"`
	SalesReportID      int      `json:"sales_report_id"`
	TestID             int      `json:"test_id"`
	Mileage            *int     `json:"mileage,omitempty"`
	CompletedDate      *string  `json:"completed_date,omitempty"`
	HasBatteryReading  *bool    `json:"has_battery_reading,omitempty"`
	BatteryResult      *float64 `json:"battery_result,omitempty"`
	AvilooBatteryScore *float64 `json:"aviloo_battery_score,omitempty"`
	AvilooReportURL    *string  `json:"aviloo_report_url,omitempty"`
	AvilooFileName     *string  `json:"aviloo_file_name,omitempty"`
	ExternalFileID     *string  `json:"external_file_id,omitempty"`
	PerformedBatteryNote string `json:"performed_battery_note,omitempty"`
}

// Client talks to BUS GraphQL endpoints.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a BUS API client.
func NewClient(c *http.Client) *Client {
	if c == nil {
		c = httputil.NewRedirectClient(map[string]string{
			"User-Agent":      httputil.UserAgent,
			"Accept-Language": "sv-SE,sv;q=0.9",
		})
	}
	return &Client{httpClient: c}
}

// ParseLink extracts tid/ts from a BUS salesReportLink URL.
func ParseLink(reportURL string) (Link, bool) {
	reportURL = strings.TrimSpace(reportURL)
	if reportURL == "" {
		return Link{}, false
	}
	u, err := url.Parse(reportURL)
	if err != nil {
		return Link{}, false
	}
	fragment := strings.TrimPrefix(u.Fragment, "/")
	if !strings.HasPrefix(fragment, "salesReportLink") {
		if strings.Contains(reportURL, "salesReportLink") {
			if idx := strings.Index(reportURL, "salesReportLink"); idx >= 0 {
				fragment = reportURL[idx:]
			}
		} else {
			return Link{}, false
		}
	}
	qIdx := strings.Index(fragment, "?")
	if qIdx < 0 {
		return Link{}, false
	}
	vals, err := url.ParseQuery(fragment[qIdx+1:])
	if err != nil {
		return Link{}, false
	}
	tid := strings.TrimSpace(vals.Get("tid"))
	ts := strings.TrimSpace(vals.Get("ts"))
	if tid == "" || ts == "" {
		return Link{}, false
	}
	return Link{TokenID: tid, TokenSecret: ts}, true
}

// ResolveReportURL follows lnk.bus.no short links to the final sales report URL.
func (c *Client) ResolveReportURL(ctx context.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.Contains(rawURL, "lnk.bus.no") {
		return rawURL, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "bus", httputil.DefaultRetryPolicy())
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return resp.Request.URL.String(), nil
}

// FetchReport resolves battery data and optional AVILOO PDF URL from a BUS link.
func (c *Client) FetchReport(ctx context.Context, reportURL string) (*Report, error) {
	resolved, err := c.ResolveReportURL(ctx, reportURL)
	if err != nil {
		return nil, err
	}
	link, ok := ParseLink(resolved)
	if !ok {
		return nil, fmt.Errorf("bus: could not parse sales report link")
	}
	session, err := c.applyToken(ctx, link)
	if err != nil {
		return nil, err
	}
	out := &Report{
		InspectionURL: resolved,
		SalesReportID: session.SalesReportID,
		TestID:        session.TestID,
	}

	var salesResp struct {
		Data struct {
			Report struct {
				TestID            int     `json:"testId"`
				Mileage           *int    `json:"mileage"`
				CompletedDate     *string `json:"completedDate"`
				HasBatteryReading *bool   `json:"hasBatteryReading"`
			} `json:"salesReport"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.graphQL(ctx, InternalGraphQL, salesReportQuery, map[string]any{"salesReportId": session.SalesReportID}, session.AccessToken, &salesResp); err == nil && len(salesResp.Errors) == 0 {
		out.Mileage = salesResp.Data.Report.Mileage
		out.CompletedDate = salesResp.Data.Report.CompletedDate
		out.HasBatteryReading = salesResp.Data.Report.HasBatteryReading
		if out.TestID <= 0 && salesResp.Data.Report.TestID > 0 {
			out.TestID = salesResp.Data.Report.TestID
		}
	}

	battery, err := c.fetchBattery(ctx, session)
	if err == nil {
		out.BatteryResult = battery.BatteryResult
		out.AvilooBatteryScore = battery.AvilooScore
		out.AvilooReportURL = battery.AvilooReportURL
		out.AvilooFileName = battery.AvilooFileName
		out.ExternalFileID = battery.ExternalFileID
		out.PerformedBatteryNote = battery.Note
	}
	if out.BatteryResult == nil && out.AvilooBatteryScore == nil && out.AvilooReportURL == nil && out.PerformedBatteryNote == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("bus: no battery data in report")
	}
	return out, nil
}

// BestSOH returns the preferred SoH percentage from a BUS report.
func BestSOH(r *Report) *float64 {
	if r == nil {
		return nil
	}
	if r.AvilooBatteryScore != nil && *r.AvilooBatteryScore > 0 && *r.AvilooBatteryScore <= 100 {
		return r.AvilooBatteryScore
	}
	if r.BatteryResult != nil && *r.BatteryResult > 0 && *r.BatteryResult <= 100 {
		return r.BatteryResult
	}
	return nil
}

// DownloadPDF fetches a certificate PDF from a SAS or public URL.
func DownloadPDF(ctx context.Context, client *http.Client, pdfURL string) ([]byte, error) {
	if client == nil {
		client = httputil.NewClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pdfURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httputil.DoWithRetry(ctx, client, req, "cert-pdf", httputil.DefaultRetryPolicy())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download pdf: %s: %s", resp.Status, string(body))
	}
	return io.ReadAll(resp.Body)
}

type session struct {
	AccessToken   string
	TestID        int
	SalesReportID int
}

type batteryData struct {
	BatteryResult   *float64
	AvilooScore     *float64
	AvilooReportURL *string
	AvilooFileName  *string
	ExternalFileID  *string
	Note            string
}

func (c *Client) applyToken(ctx context.Context, link Link) (session, error) {
	var resp struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"accessToken"`
			} `json:"tokens"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.graphQL(ctx, SecurityGraphQL, applyTokenQuery, map[string]any{
		"tokenId": link.TokenID, "tokenSecret": link.TokenSecret, "applicationId": SalesReportApp,
	}, "", &resp); err != nil {
		return session{}, err
	}
	if len(resp.Errors) > 0 {
		return session{}, fmt.Errorf("bus apply token: %v", resp.Errors)
	}
	payload, err := decodeJWTClaims(resp.Data.Tokens.AccessToken)
	if err != nil {
		return session{}, err
	}
	inner, err := businessTokenFromClaims(payload)
	if err != nil {
		return session{}, err
	}
	return session{
		AccessToken:   resp.Data.Tokens.AccessToken,
		TestID:        intFromJSON(inner["testId"]),
		SalesReportID: intFromJSON(inner["salesReportId"]),
	}, nil
}

func (c *Client) fetchBattery(ctx context.Context, session session) (batteryData, error) {
	if session.TestID <= 0 || session.AccessToken == "" {
		return batteryData{}, fmt.Errorf("bus: missing test id or token")
	}
	out := batteryData{}
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
	if err := c.graphQL(ctx, InternalGraphQL, batteryMetaQuery, map[string]any{"testId": session.TestID}, session.AccessToken, &metaResp); err == nil && len(metaResp.Errors) == 0 {
		if metaResp.Data.Meta.IsBatteryReadingExcludeFromReport {
			return batteryData{}, fmt.Errorf("bus: battery reading excluded from report")
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
	if err := c.graphQL(ctx, InternalGraphQL, avilooScoreQuery, map[string]any{"testId": session.TestID}, session.AccessToken, &scoreResp); err == nil && len(scoreResp.Errors) == 0 {
		out.AvilooScore = scoreResp.Data.Score
	}

	var reportResp struct {
		Data struct {
			Report *struct {
				ExternalFileID string `json:"externalFileId"`
				FileName       string `json:"fileName"`
				SasURL         string `json:"sasUrl"`
			} `json:"avilooReport"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := c.graphQL(ctx, InternalGraphQL, avilooReportQuery, map[string]any{"testId": session.TestID}, session.AccessToken, &reportResp); err == nil && len(reportResp.Errors) == 0 {
		if reportResp.Data.Report != nil {
			if u := strings.TrimSpace(reportResp.Data.Report.SasURL); u != "" {
				out.AvilooReportURL = &u
			}
			if fn := strings.TrimSpace(reportResp.Data.Report.FileName); fn != "" {
				out.AvilooFileName = &fn
			}
			if id := strings.TrimSpace(reportResp.Data.Report.ExternalFileID); id != "" {
				out.ExternalFileID = &id
			}
		}
	}

	if out.BatteryResult == nil && out.AvilooScore == nil && out.Note == "" && out.AvilooReportURL == nil {
		return batteryData{}, fmt.Errorf("bus: no battery data")
	}
	return out, nil
}

func (c *Client) graphQL(ctx context.Context, endpoint, query string, variables map[string]any, accessToken string, out any) error {
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
	req.Header.Set("Origin", "https://bus2.bus.no")
	req.Header.Set("Referer", "https://bus2.bus.no/BUSPlatform3/BUStest.Client/")
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
