package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type UserAttribute struct {
	ID                *float64 `json:"id,omitempty"`
	Name              string   `json:"name"`
	AttributeType     string   `json:"attribute_type"`
	Label             string   `json:"label"`
	Description       *string  `json:"description,omitempty"`
	IsSystemAttribute *bool    `json:"is_system_attribute,omitempty"`
}

type UserAttributeInput struct {
	Name          string  `json:"name"`
	AttributeType string  `json:"attribute_type"`
	Label         string  `json:"label"`
	Description   *string `json:"description,omitempty"`
}

type userAttributeEnvelope struct {
	UserAttribute UserAttribute `json:"user_attribute"`
}

type userAttributeInputEnvelope struct {
	UserAttribute UserAttributeInput `json:"user_attribute"`
}

func (c *Client) CreateUserAttribute(ctx context.Context, input UserAttributeInput) (*UserAttribute, error) {
	body := userAttributeInputEnvelope{UserAttribute: input}
	var out userAttributeEnvelope
	if err := c.Do(ctx, http.MethodPost, "/user_attributes", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.UserAttribute, nil
}

func (c *Client) UpdateUserAttribute(ctx context.Context, id int, input UserAttributeInput) (*UserAttribute, error) {
	body := userAttributeInputEnvelope{UserAttribute: input}
	var out userAttributeEnvelope
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/user_attributes/%d", id), nil, body, &out); err != nil {
		return nil, err
	}
	return &out.UserAttribute, nil
}

func (c *Client) DeleteUserAttribute(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/user_attributes/%d", id), nil, nil, nil)
}

// GetUserAttribute paginates /user_attributes and returns the entry matching id.
// The API has no GET /user_attributes/{id} endpoint, so we filter the list.
func (c *Client) GetUserAttribute(ctx context.Context, id int) (*UserAttribute, error) {
	var after string
	for {
		q := url.Values{}
		q.Set("limit", "100")
		if after != "" {
			q.Set("after", after)
		}
		var page struct {
			UserAttributes []UserAttribute `json:"user_attributes"`
			Cursors        struct {
				Next *string `json:"next"`
			} `json:"cursors"`
		}
		if err := c.Do(ctx, http.MethodGet, "/user_attributes", q, nil, &page); err != nil {
			return nil, err
		}
		for i := range page.UserAttributes {
			ua := page.UserAttributes[i]
			if ua.ID != nil && int(*ua.ID) == id {
				return &ua, nil
			}
		}
		if page.Cursors.Next == nil || *page.Cursors.Next == "" {
			break
		}
		after = *page.Cursors.Next
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Message: "user_attribute " + strconv.Itoa(id) + " not found"}
}
