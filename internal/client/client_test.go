package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		maxWait time.Duration
		want    time.Duration
		approx  bool // for HTTP-date case where time.Until is approximate
	}{
		{"seconds", "5", time.Minute, 5 * time.Second, false},
		{"seconds clamped", "120", 30 * time.Second, 30 * time.Second, false},
		{"whitespace seconds", "  10  ", time.Minute, 10 * time.Second, false},
		{"empty defaults to 1s", "", time.Minute, time.Second, false},
		{"garbage defaults to 1s", "soon", time.Minute, time.Second, false},
		{"http date in future", time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat), time.Minute, 3 * time.Second, true},
		{"http date in past clamps to 0", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), time.Minute, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseRetryAfter(c.value, c.maxWait)
			if c.approx {
				delta := got - c.want
				if delta < -2*time.Second || delta > time.Second {
					t.Fatalf("got %v, want ~%v", got, c.want)
				}
				return
			}
			if got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// Returns 429 the first N times, then succeeds. Verifies the client honors
// Retry-After (in seconds form) and retries up to MaxRetries.
func TestClient_RetryOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"type":"RateLimitExceedError","message":"rate limit hit"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c, err := New(Options{APIKey: "x", BaseURL: srv.URL, MaxRetries: 3, MaxRetryWait: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var out map[string]any
	if err := c.Do(context.Background(), http.MethodGet, "/anything", nil, nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (2 retries + success), got %d", got)
	}
}

// After exhausting retries the client should return an APIError with
// RetryAfter populated so callers can surface it.
func TestClient_RetryExhaustedReturns429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `{"type":"RateLimitExceedError","message":"still rate limited"}`)
	}))
	defer srv.Close()

	c, err := New(Options{APIKey: "x", BaseURL: srv.URL, MaxRetries: 1, MaxRetryWait: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	err = c.Do(context.Background(), http.MethodGet, "/anything", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status: got %d, want 429", apiErr.StatusCode)
	}
	if apiErr.RetryAfter != "42" {
		t.Fatalf("RetryAfter: got %q, want %q", apiErr.RetryAfter, "42")
	}
}

// Multiple GetUser calls within the cache TTL should only hit /users once.
// This is the win for users with many `data "holistics_user"` blocks.
func TestClient_UserCacheCoalescesReads(t *testing.T) {
	var listCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users" {
			listCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"users":[
				{"id":1,"email":"a@example.com","initials":"A","role":"admin","is_deleted":false,"is_activated":true,"has_authentication_token":false,"allow_authentication_token":true,"enable_export_data":true,"created_at":"2026-01-01T00:00:00Z"},
				{"id":2,"email":"b@example.com","initials":"B","role":"user","is_deleted":false,"is_activated":true,"has_authentication_token":false,"allow_authentication_token":true,"enable_export_data":true,"created_at":"2026-01-01T00:00:00Z"}
			],"cursors":{"next":null}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := New(Options{APIKey: "x", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// 5 reads — by ID, by email, mix.
	if _, err := c.GetUser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetUser(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetUserByEmail(ctx, "a@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetUserByEmail(ctx, "b@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetUser(ctx, 1); err != nil {
		t.Fatal(err)
	}

	if got := listCalls.Load(); got != 1 {
		t.Fatalf("expected /users hit once, got %d", got)
	}
}

// A write should invalidate the user cache so the next read sees fresh data.
func TestClient_UserCacheInvalidatesOnWrite(t *testing.T) {
	var listCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users" && r.Method == http.MethodGet:
			listCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"users":[
				{"id":1,"email":"a@example.com","initials":"A","role":"admin","is_deleted":false,"is_activated":true,"has_authentication_token":false,"allow_authentication_token":true,"enable_export_data":true,"created_at":"2026-01-01T00:00:00Z"}
			],"cursors":{"next":null}}`)
		case r.URL.Path == "/users/1" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"id":1,"email":"a@example.com","initials":"A","role":"admin","is_deleted":false,"is_activated":true,"has_authentication_token":false,"allow_authentication_token":true,"enable_export_data":true,"created_at":"2026-01-01T00:00:00Z"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New(Options{APIKey: "x", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := c.GetUser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	// Without invalidation a 2nd GetUser would use cache (1 list call total).
	// We do a write between the two — expect a second list call after that.
	if _, err := c.UpdateUser(ctx, 1, UpdateUserInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetUser(ctx, 1); err != nil {
		t.Fatal(err)
	}

	if got := listCalls.Load(); got != 2 {
		t.Fatalf("expected /users hit twice (cache invalidated by write), got %d", got)
	}
}
