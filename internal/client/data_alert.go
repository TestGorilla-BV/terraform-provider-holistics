package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type DataAlertEmailOptions struct {
	BodyText *string `json:"body_text,omitempty"`
}

type DataAlertEmailDestFields struct {
	Title      *string                `json:"title,omitempty"`
	Recipients []string               `json:"recipients,omitempty"`
	Options    *DataAlertEmailOptions `json:"options,omitempty"`
}

type DataAlertSlackChannel struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

type DataAlertSlackDestFields struct {
	Title         *string                 `json:"title,omitempty"`
	Message       *string                 `json:"message,omitempty"`
	SlackChannels []DataAlertSlackChannel `json:"slack_channels,omitempty"`
}

type DataAlertWebhookDestFields struct {
	Endpoint string `json:"endpoint"`
}

type DataAlertDest struct {
	Type    string                      `json:"type"`
	Email   *DataAlertEmailDestFields   `json:",inline"`
	Slack   *DataAlertSlackDestFields   `json:",inline"`
	Webhook *DataAlertWebhookDestFields `json:",inline"`
}

func (d DataAlertDest) MarshalJSON() ([]byte, error) {
	out := map[string]any{"type": d.Type}
	switch d.Type {
	case "EmailDest":
		if d.Email != nil {
			mergeStruct(out, d.Email)
		}
	case "SlackDest":
		if d.Slack != nil {
			mergeStruct(out, d.Slack)
		}
	case "WebhookDest":
		if d.Webhook != nil {
			mergeStruct(out, d.Webhook)
		}
	}
	return json.Marshal(out)
}

func (d *DataAlertDest) UnmarshalJSON(data []byte) error {
	var typeOnly struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeOnly); err != nil {
		return err
	}
	d.Type = typeOnly.Type
	switch d.Type {
	case "EmailDest":
		d.Email = &DataAlertEmailDestFields{}
		return json.Unmarshal(data, d.Email)
	case "SlackDest":
		d.Slack = &DataAlertSlackDestFields{}
		return json.Unmarshal(data, d.Slack)
	case "WebhookDest":
		d.Webhook = &DataAlertWebhookDestFields{}
		return json.Unmarshal(data, d.Webhook)
	}
	return nil
}

type FieldPath struct {
	FieldName string     `json:"field_name"`
	ModelID   FlexibleID `json:"model_id,omitempty"`
	DataSetID *int       `json:"data_set_id,omitempty"`
	IsMetric  *bool      `json:"is_metric,omitempty"`
}

type VizCondition struct {
	ID             *int      `json:"id,omitempty"`
	FieldPath      FieldPath `json:"field_path"`
	Aggregation    *string   `json:"aggregation,omitempty"`
	Transformation *string   `json:"transformation,omitempty"`
	Condition      Condition `json:"condition"`
}

type DataAlert struct {
	ID                   *int                  `json:"id,omitempty"`
	Title                *string               `json:"title,omitempty"`
	SourceID             FlexibleID            `json:"source_id"`
	SourceType           string                `json:"source_type"`
	Dest                 DataAlertDest         `json:"dest"`
	DynamicFilterPresets []DynamicFilterPreset `json:"dynamic_filter_presets"`
	VizConditions        []VizCondition        `json:"viz_conditions,omitempty"`
	Schedule             Schedule              `json:"schedule"`
}

type dataAlertEnvelope struct {
	DataAlert DataAlert `json:"data_alert"`
}

func (c *Client) CreateDataAlert(ctx context.Context, input DataAlert) (*DataAlert, error) {
	body := dataAlertEnvelope{DataAlert: input}
	var out dataAlertEnvelope
	if err := c.Do(ctx, http.MethodPost, "/data_alerts", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.DataAlert, nil
}

func (c *Client) GetDataAlert(ctx context.Context, id int) (*DataAlert, error) {
	var out dataAlertEnvelope
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/data_alerts/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.DataAlert, nil
}

func (c *Client) UpdateDataAlert(ctx context.Context, id int, input DataAlert) (*DataAlert, error) {
	body := dataAlertEnvelope{DataAlert: input}
	var out dataAlertEnvelope
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/data_alerts/%d", id), nil, body, &out); err != nil {
		return nil, err
	}
	return &out.DataAlert, nil
}

func (c *Client) DeleteDataAlert(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/data_alerts/%d", id), nil, nil, nil)
}
