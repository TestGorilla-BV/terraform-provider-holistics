package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	RegionAPAC = "apac"
	RegionUS   = "us"
	RegionEU   = "eu"
)

var regionBaseURLs = map[string]string{
	RegionAPAC: "https://secure.holistics.io/api/v2",
	RegionUS:   "https://us.holistics.io/api/v2",
	RegionEU:   "https://eu.holistics.io/api/v2",
}

type Client struct {
	apiKey       string
	baseURL      *url.URL
	httpClient   *http.Client
	userAgent    string
	maxRetries   int
	maxRetryWait time.Duration
	cacheTTL     time.Duration

	usersCache       listCache[User]
	tagsCache        listCache[Tag]
	currentUserCache singleCache[User]
}

type Options struct {
	APIKey    string
	Region    string
	BaseURL   string
	UserAgent string
	Timeout   time.Duration

	// MaxRetries is how many additional attempts to make after a 429 response.
	// Zero disables retry. Default: 3.
	MaxRetries int
	// MaxRetryWait caps how long we sleep for any single Retry-After header.
	// Holistics' default tier resets every minute, so 60s is usually enough.
	// Default: 60s.
	MaxRetryWait time.Duration
	// CacheTTL controls how long the in-memory caches (users list, tags list,
	// current_user) live before being refetched. Set to 0 to disable.
	// Default: 30s — long enough to coalesce all reads inside a single
	// terraform plan/apply phase, short enough to pick up external changes.
	CacheTTL time.Duration
}

func New(opts Options) (*Client, error) {
	if opts.APIKey == "" {
		return nil, errors.New("api_key is required")
	}

	raw := opts.BaseURL
	if raw == "" {
		region := strings.ToLower(opts.Region)
		if region == "" {
			region = RegionAPAC
		}
		base, ok := regionBaseURLs[region]
		if !ok {
			return nil, fmt.Errorf("unknown region %q (expected apac, us, or eu)", region)
		}
		raw = base
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid base url %q: %w", raw, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	maxRetries := opts.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	maxRetryWait := opts.MaxRetryWait
	if maxRetryWait == 0 {
		maxRetryWait = 60 * time.Second
	}
	cacheTTL := opts.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 30 * time.Second
	}

	c := &Client{
		apiKey:       opts.APIKey,
		baseURL:      u,
		httpClient:   &http.Client{Timeout: timeout},
		userAgent:    opts.UserAgent,
		maxRetries:   maxRetries,
		maxRetryWait: maxRetryWait,
		cacheTTL:     cacheTTL,
	}
	c.usersCache.ttl = cacheTTL
	c.tagsCache.ttl = cacheTTL
	c.currentUserCache.ttl = cacheTTL
	return c, nil
}

type APIError struct {
	StatusCode int
	Type       string
	Message    string
	Body       string
	// RetryAfter is set when StatusCode is 429 (Too Many Requests). Contains
	// the raw Retry-After header value; format is either delta-seconds or an
	// HTTP-date per RFC 7231.
	RetryAfter string
}

func (e *APIError) Error() string {
	if e.Type != "" || e.Message != "" {
		return fmt.Sprintf("holistics api error (status %d, type=%s): %s", e.StatusCode, e.Type, e.Message)
	}
	return fmt.Sprintf("holistics api error (status %d): %s", e.StatusCode, e.Body)
}

func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}
	return false
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyBytes = b
	}

	for attempt := 0; ; attempt++ {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("X-Holistics-Key", c.apiKey)
		req.Header.Set("Accept", "application/json")
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"), c.maxRetryWait)
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if resp.StatusCode >= 400 {
			apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
			var parsed struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if json.Unmarshal(respBody, &parsed) == nil {
				apiErr.Type = parsed.Type
				apiErr.Message = parsed.Message
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				apiErr.RetryAfter = resp.Header.Get("Retry-After")
			}
			return apiErr
		}

		if out == nil || len(respBody) == 0 {
			return nil
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
}

// parseRetryAfter handles both forms of the Retry-After header (RFC 7231 §7.1.3):
//   - delta-seconds: "120"
//   - HTTP-date: "Wed, 21 Oct 2026 07:28:00 GMT"
//
// Result is clamped to [0, maxWait]. Falls back to 1s if the header is missing
// or unparseable so we don't tight-loop.
func parseRetryAfter(value string, maxWait time.Duration) time.Duration {
	clamp := func(d time.Duration) time.Duration {
		if d < 0 {
			return 0
		}
		if d > maxWait {
			return maxWait
		}
		return d
	}
	if value == "" {
		return time.Second
	}
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return clamp(time.Duration(n) * time.Second)
	}
	if t, err := http.ParseTime(value); err == nil {
		return clamp(time.Until(t))
	}
	return time.Second
}

// listCache is a TTL-bounded cache for slice-shaped list endpoints.
type listCache[T any] struct {
	mu        sync.Mutex
	value     []T
	fetchedAt time.Time
	ttl       time.Duration
}

func (c *listCache[T]) get(ctx context.Context, fetch func(context.Context) ([]T, error)) ([]T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ttl > 0 && c.value != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.value, nil
	}
	v, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.value = v
	c.fetchedAt = time.Now()
	return v, nil
}

func (c *listCache[T]) invalidate() {
	c.mu.Lock()
	c.value = nil
	c.fetchedAt = time.Time{}
	c.mu.Unlock()
}

// singleCache is the same idea but for a single-value endpoint like /users/me.
type singleCache[T any] struct {
	mu        sync.Mutex
	value     *T
	fetchedAt time.Time
	ttl       time.Duration
}

func (c *singleCache[T]) get(ctx context.Context, fetch func(context.Context) (*T, error)) (*T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ttl > 0 && c.value != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.value, nil
	}
	v, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.value = v
	c.fetchedAt = time.Now()
	return v, nil
}

func (c *singleCache[T]) invalidate() {
	c.mu.Lock()
	c.value = nil
	c.fetchedAt = time.Time{}
	c.mu.Unlock()
}
