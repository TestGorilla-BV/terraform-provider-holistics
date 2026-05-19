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
	"strings"
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
	apiKey     string
	baseURL    *url.URL
	httpClient *http.Client
	userAgent  string
}

type Options struct {
	APIKey    string
	Region    string
	BaseURL   string
	UserAgent string
	Timeout   time.Duration
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

	return &Client{
		apiKey:     opts.APIKey,
		baseURL:    u,
		httpClient: &http.Client{Timeout: timeout},
		userAgent:  opts.UserAgent,
	}, nil
}

type APIError struct {
	StatusCode int
	Type       string
	Message    string
	Body       string
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

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
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
