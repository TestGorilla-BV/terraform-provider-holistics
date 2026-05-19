package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type Group struct {
	ID      *int   `json:"id,omitempty"`
	Name    string `json:"name"`
	UserIDs []int  `json:"user_ids,omitempty"`
}

type groupEnvelope struct {
	Group Group `json:"group"`
}

type groupInput struct {
	Name string `json:"name"`
}

type groupInputEnvelope struct {
	Group groupInput `json:"group"`
}

func (c *Client) CreateGroup(ctx context.Context, name string) (*Group, error) {
	body := groupInputEnvelope{Group: groupInput{Name: name}}
	var out groupEnvelope
	if err := c.Do(ctx, http.MethodPost, "/groups", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Group, nil
}

func (c *Client) GetGroup(ctx context.Context, id int) (*Group, error) {
	var out groupEnvelope
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/groups/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Group, nil
}

func (c *Client) UpdateGroup(ctx context.Context, id int, name string) (*Group, error) {
	body := groupInputEnvelope{Group: groupInput{Name: name}}
	var out groupEnvelope
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/groups/%d", id), nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Group, nil
}

func (c *Client) DeleteGroup(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/groups/%d", id), nil, nil, nil)
}

func (c *Client) AddGroupUser(ctx context.Context, groupID, userID int) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("/groups/%d/add_user/%d", groupID, userID), nil, nil, nil)
}

func (c *Client) RemoveGroupUser(ctx context.Context, groupID, userID int) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("/groups/%d/remove_user/%d", groupID, userID), nil, nil, nil)
}

// GetGroupWithUsers fetches a group including the populated user_ids list.
func (c *Client) GetGroupWithUsers(ctx context.Context, id int) (*Group, error) {
	q := url.Values{}
	q.Set("include_users", "true")
	var out groupEnvelope
	path := fmt.Sprintf("/groups/%d", id)
	if err := c.Do(ctx, http.MethodGet, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Group, nil
}
