package kvd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/schema"
)

const baseURL = "https://www.kvd.se"

var candidatePaths = []string{
	"/api/vehicles",
	"/api/search",
	"/api/v1/vehicles",
}

// UnavailableError indicates no public KVD API endpoint responded.
type UnavailableError struct{}

func (UnavailableError) Error() string {
	return "no public KVD API endpoint responded with JSON"
}

// Client probes KVD for a public JSON API.
type Client struct {
	httpClient *http.Client
	owns       bool
	available  *bool
}

func NewClient(c *http.Client) *Client {
	if c == nil {
		c = httputil.NewClient()
	}
	return &Client{httpClient: c, owns: true}
}

func (c *Client) Probe(ctx context.Context) bool {
	if c.available != nil {
		return *c.available
	}
	for _, path := range candidatePaths {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			continue
		}
		q := req.URL.Query()
		q.Set("limit", "1")
		req.URL.RawQuery = q.Encode()
		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 && strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			v := true
			c.available = &v
			return true
		}
	}
	v := false
	c.available = &v
	return false
}

func (c *Client) Search(ctx context.Context) ([]schema.CarListing, error) {
	if !c.Probe(ctx) {
		return nil, UnavailableError{}
	}
	return nil, nil
}

func (c *Client) Close() {
	if c.owns && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

func (c *Client) String() string { return fmt.Sprintf("kvd.Client") }
