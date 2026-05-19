package client

import (
	"context"
	"net/http"
)

type Tag struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
}

func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	return c.tagsCache.get(ctx, func(ctx context.Context) ([]Tag, error) {
		var out struct {
			Tags []Tag `json:"tags"`
		}
		if err := c.Do(ctx, http.MethodGet, "/tags", nil, nil, &out); err != nil {
			return nil, err
		}
		return out.Tags, nil
	})
}
