package httputil_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/httputil"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDoWithRetrySucceedsAfter429(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n <= 2 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	policy := httputil.RetryPolicy{MaxRetries: 2, MaxWait: time.Second, DefaultWait: time.Millisecond}
	resp, err := httputil.DoWithRetry(context.Background(), client, req, "test", policy)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestDoWithRetryReturnsRateLimitedError(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"120"}},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	policy := httputil.RetryPolicy{MaxRetries: 1, MaxWait: time.Millisecond, DefaultWait: time.Millisecond}
	_, err := httputil.DoWithRetry(context.Background(), client, req, "carla", policy)
	if err == nil {
		t.Fatal("expected error")
	}
	if !httputil.IsRateLimited(err) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "carla") {
		t.Fatalf("err = %v", err)
	}
}

func TestDoWithRetryDoesNotRetryOtherErrors(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	resp, err := httputil.DoWithRetry(context.Background(), client, req, "test", httputil.DefaultRetryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
}
