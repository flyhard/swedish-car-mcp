package blocketproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/blocket"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
)

const defaultProxyBase = "https://blocket-api.se"

// ProxyBaseURL returns the configured Blocket proxy base URL.
func ProxyBaseURL() string {
	if v := os.Getenv("BLOCKET_PROXY_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultProxyBase
}

// Client queries Blocket via an optional proxy service.
type Client struct {
	httpClient *http.Client
	owns       bool
	baseURL    string
}

func NewClient(c *http.Client, baseURL string) *Client {
	if c == nil {
		c = httputil.NewClient()
	}
	if baseURL == "" {
		baseURL = ProxyBaseURL()
	}
	return &Client{httpClient: c, owns: c.Transport == nil, baseURL: strings.TrimRight(baseURL, "/")}
}

func (c *Client) Search(ctx context.Context, params map[string]string) ([]schema.CarListing, error) {
	u := c.baseURL + "/search"
	reqURL, _ := url.Parse(u)
	q := reqURL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	reqURL.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httputil.DoWithRetry(ctx, c.httpClient, req, "blocket", httputil.DefaultRetryPolicy())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	docs, _ := payload["docs"].([]any)
	if docs == nil {
		if results, ok := payload["results"].([]any); ok {
			docs = results
		} else if arr, ok := payload[""].([]any); ok {
			docs = arr
		}
	}
	if docs == nil {
		if arr, ok := any(payload).([]any); ok {
			docs = arr
		}
	}
	if docs == nil {
		return nil, nil
	}
	out := make([]schema.CarListing, 0, len(docs))
	for _, doc := range docs {
		if m, ok := doc.(map[string]any); ok {
			out = append(out, blocket.ParseAd(m))
		}
	}
	return out, nil
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}
