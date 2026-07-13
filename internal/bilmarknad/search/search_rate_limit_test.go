package search

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/carla"
	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
)

type rateLimitTransport func(*http.Request) (*http.Response, error)

func (f rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSearchCarsSkipsRateLimitedCarla(t *testing.T) {
	carlaClient := carla.NewClient(&http.Client{Transport: rateLimitTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"0"}},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})})
	defer carlaClient.Close()

	svc := &Service{carla: carlaClient}
	results, err := svc.SearchCars(context.Background(), SearchOptions{
		Sources: []string{"carla"},
		Limit:   5,
	})
	if err != nil {
		t.Fatalf("SearchCars() err = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %d, want 0", len(results))
	}
}

func TestIsSkippableSourceErrRateLimited(t *testing.T) {
	if !isSkippableSourceErr(httputil.RateLimitedError{Source: "carla"}) {
		t.Fatal("expected rate limited error to be skippable")
	}
}
