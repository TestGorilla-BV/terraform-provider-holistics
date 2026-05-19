package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type RowBasedPermissionRule struct {
	Condition Condition `json:"condition"`
	FieldPath FieldPath `json:"field_path"`
}

type PermissionRules struct {
	RowBased []RowBasedPermissionRule `json:"row_based,omitempty"`
}

type ShareableLink struct {
	ID                   json.Number           `json:"id,omitempty"`
	ResourceType         string                `json:"resource_type"`
	ResourceID           json.Number           `json:"resource_id"`
	Title                *string               `json:"title,omitempty"`
	PasswordEnabled      *bool                 `json:"password_enabled,omitempty"`
	Password             *string               `json:"password,omitempty"`
	ExpiredAt            *string               `json:"expired_at,omitempty"`
	DynamicFilterPresets []DynamicFilterPreset `json:"dynamic_filter_presets,omitempty"`
	PermissionRules      *PermissionRules      `json:"permission_rules,omitempty"`
}

type shareableLinkEnvelope struct {
	ShareableLink ShareableLink `json:"shareable_link"`
}

func (c *Client) CreateShareableLink(ctx context.Context, input ShareableLink) (*ShareableLink, error) {
	body := shareableLinkEnvelope{ShareableLink: input}
	var out shareableLinkEnvelope
	if err := c.Do(ctx, http.MethodPost, "/shareable_links", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.ShareableLink, nil
}

func (c *Client) GetShareableLink(ctx context.Context, id string) (*ShareableLink, error) {
	var out shareableLinkEnvelope
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/shareable_links/%s", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.ShareableLink, nil
}

func (c *Client) UpdateShareableLink(ctx context.Context, id string, input ShareableLink) (*ShareableLink, error) {
	body := shareableLinkEnvelope{ShareableLink: input}
	var out shareableLinkEnvelope
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/shareable_links/%s", id), nil, body, &out); err != nil {
		return nil, err
	}
	return &out.ShareableLink, nil
}

func (c *Client) DeleteShareableLink(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/shareable_links/%s", id), nil, nil, nil)
}
