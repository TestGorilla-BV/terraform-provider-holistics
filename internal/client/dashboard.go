package client

import (
	"context"
	"fmt"
	"net/http"
)

type Dashboard struct {
	ID         int      `json:"id"`
	OwnerID    *int     `json:"owner_id,omitempty"`
	Title      string   `json:"title"`
	CategoryID *int     `json:"category_id,omitempty"`
	Version    *int     `json:"version,omitempty"`
	URL        *string  `json:"url,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func (c *Client) GetDashboard(ctx context.Context, id int) (*Dashboard, error) {
	var out struct {
		Dashboard Dashboard `json:"dashboard"`
	}
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/dashboards/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Dashboard, nil
}
