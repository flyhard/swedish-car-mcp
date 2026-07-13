package httputil

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitedError indicates the upstream returned HTTP 429 after retries were exhausted.
type RateLimitedError struct {
	Source     string
	RetryAfter time.Duration
}

func (e RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s: rate limited (retry after %s)", e.Source, e.RetryAfter)
	}
	return fmt.Sprintf("%s: rate limited", e.Source)
}

// IsRateLimited reports whether err is a RateLimitedError.
func IsRateLimited(err error) bool {
	var rl RateLimitedError
	return errors.As(err, &rl)
}

// RetryPolicy configures bounded retries for HTTP 429 responses.
type RetryPolicy struct {
	MaxRetries  int
	MaxWait     time.Duration
	DefaultWait time.Duration
}

// DefaultRetryPolicy returns the standard bilmarknad retry settings.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:  2,
		MaxWait:     30 * time.Second,
		DefaultWait: time.Second,
	}
}

// DoWithRetry executes req and retries on HTTP 429, honoring Retry-After when present.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, source string, policy RetryPolicy) (*http.Response, error) {
	if policy.MaxRetries < 0 {
		policy.MaxRetries = 0
	}
	if policy.MaxWait <= 0 {
		policy.MaxWait = 30 * time.Second
	}
	if policy.DefaultWait <= 0 {
		policy.DefaultWait = time.Second
	}

	var lastRetryAfter time.Duration
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		attemptReq := req
		if attempt > 0 {
			attemptReq = req.Clone(ctx)
			if attemptReq == nil {
				return nil, fmt.Errorf("failed to clone request")
			}
		}

		resp, err := client.Do(attemptReq)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		lastRetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		resp.Body.Close()

		if attempt == policy.MaxRetries {
			return nil, RateLimitedError{Source: source, RetryAfter: lastRetryAfter}
		}

		wait := lastRetryAfter
		if wait <= 0 {
			wait = policy.DefaultWait
		}
		if wait > policy.MaxWait {
			wait = policy.MaxWait
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, RateLimitedError{Source: source, RetryAfter: lastRetryAfter}
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(v); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}
