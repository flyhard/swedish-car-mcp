package httputil

import (
	"net/http"
	"time"
)

const UserAgent = "bilmarknad-mcp/0.2"

// NewClient returns an HTTP client with the default bilmarknad user agent.
func NewClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &userAgentTransport{
			base: http.DefaultTransport,
			ua:   UserAgent,
		},
	}
}

type userAgentTransport struct {
	base http.RoundTripper
	ua   string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("User-Agent", t.ua)
	return t.base.RoundTrip(cloned)
}

// NewRedirectClient returns a client that follows redirects.
func NewRedirectClient(headers map[string]string) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &headerTransport{
			base:    http.DefaultTransport,
			headers: headers,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	for k, v := range t.headers {
		cloned.Header.Set(k, v)
	}
	return t.base.RoundTrip(cloned)
}
