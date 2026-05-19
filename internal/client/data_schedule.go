package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DataScheduleDest is the polymorphic destination — exactly one nested struct
// should be populated. The `type` discriminator is set by callers/decoders.
type DataScheduleDest struct {
	Type              string                                   `json:"type"`
	Email             *DataScheduleEmailDestFields             `json:",inline"`
	EmailSubscription *DataScheduleEmailSubscriptionDestFields `json:",inline"`
	Sftp              *DataScheduleSftpDestFields              `json:",inline"`
	Slack             *DataScheduleSlackDestFields             `json:",inline"`
	GoogleSheet       *DataScheduleGoogleSheetDestFields       `json:",inline"`
}

type DataScheduleEmailOptions struct {
	Preview           *bool         `json:"preview,omitempty"`
	IncludeHeader     *bool         `json:"include_header,omitempty"`
	IncludeReportLink *bool         `json:"include_report_link,omitempty"`
	IncludeFilters    *bool         `json:"include_filters,omitempty"`
	DontSendWhenEmpty *bool         `json:"dont_send_when_empty,omitempty"`
	BodyText          *string       `json:"body_text,omitempty"`
	SelectedType      *string       `json:"selected_type,omitempty"`
	SelectedValues    []json.Number `json:"selected_values,omitempty"`
	AttachmentFormats []string      `json:"attachment_formats,omitempty"`
}

type DataScheduleEmailDestFields struct {
	Title      *string                   `json:"title,omitempty"`
	Recipients []string                  `json:"recipients,omitempty"`
	Options    *DataScheduleEmailOptions `json:"options,omitempty"`
}

type DataScheduleEmailSubscriptionDestFields struct {
	Recipients []string                  `json:"recipients,omitempty"`
	Options    *DataScheduleEmailOptions `json:"options,omitempty"`
}

type DataScheduleSftpDestFields struct {
	DashboardWidgetID *string `json:"dashboard_widget_id,omitempty"`
	DataSourceID      int     `json:"data_source_id"`
	FilePath          string  `json:"file_path"`
	Format            *string `json:"format,omitempty"`
	IncludeHeader     *bool   `json:"include_header,omitempty"`
	Separator         *string `json:"separator,omitempty"`
}

type SlackChannel struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
	Type *string `json:"type,omitempty"`
}

type DataScheduleSlackDestFields struct {
	HideFilters   *bool          `json:"hide_filters,omitempty"`
	Message       *string        `json:"message,omitempty"`
	SlackChannels []SlackChannel `json:"slack_channels,omitempty"`
	Title         *string        `json:"title,omitempty"`
}

type DataScheduleGoogleSheetDestFields struct {
	SheetURL   string `json:"sheet_url"`
	SheetTitle string `json:"sheet_title"`
}

func (d DataScheduleDest) MarshalJSON() ([]byte, error) {
	out := map[string]any{"type": d.Type}
	switch d.Type {
	case "EmailDest":
		if d.Email != nil {
			mergeStruct(out, d.Email)
		}
	case "EmailSubscriptionDest":
		if d.EmailSubscription != nil {
			mergeStruct(out, d.EmailSubscription)
		}
	case "SftpDest":
		if d.Sftp != nil {
			mergeStruct(out, d.Sftp)
		}
	case "SlackDest":
		if d.Slack != nil {
			mergeStruct(out, d.Slack)
		}
	case "GsheetDest":
		if d.GoogleSheet != nil {
			mergeStruct(out, d.GoogleSheet)
		}
	}
	return json.Marshal(out)
}

func (d *DataScheduleDest) UnmarshalJSON(data []byte) error {
	var typeOnly struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeOnly); err != nil {
		return err
	}
	d.Type = typeOnly.Type
	switch d.Type {
	case "EmailDest":
		d.Email = &DataScheduleEmailDestFields{}
		return json.Unmarshal(data, d.Email)
	case "EmailSubscriptionDest":
		d.EmailSubscription = &DataScheduleEmailSubscriptionDestFields{}
		return json.Unmarshal(data, d.EmailSubscription)
	case "SftpDest":
		d.Sftp = &DataScheduleSftpDestFields{}
		return json.Unmarshal(data, d.Sftp)
	case "SlackDest":
		d.Slack = &DataScheduleSlackDestFields{}
		return json.Unmarshal(data, d.Slack)
	case "GsheetDest":
		d.GoogleSheet = &DataScheduleGoogleSheetDestFields{}
		return json.Unmarshal(data, d.GoogleSheet)
	}
	return nil
}

// mergeStruct marshals v to JSON and copies its top-level keys into out.
func mergeStruct(out map[string]any, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return
	}
	for k, val := range m {
		out[k] = val
	}
}

type DataSchedule struct {
	ID                   *int                  `json:"id,omitempty"`
	Title                *string               `json:"title,omitempty"`
	SourceType           string                `json:"source_type"`
	SourceID             int                   `json:"source_id"`
	Schedule             Schedule              `json:"schedule"`
	DynamicFilterPresets []DynamicFilterPreset `json:"dynamic_filter_presets,omitempty"`
	Dest                 DataScheduleDest      `json:"dest"`
}

type dataScheduleEnvelope struct {
	DataSchedule DataSchedule `json:"data_schedule"`
}

func (c *Client) CreateDataSchedule(ctx context.Context, input DataSchedule) (*DataSchedule, error) {
	body := dataScheduleEnvelope{DataSchedule: input}
	var out dataScheduleEnvelope
	if err := c.Do(ctx, http.MethodPost, "/data_schedules", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.DataSchedule, nil
}

func (c *Client) GetDataSchedule(ctx context.Context, id int) (*DataSchedule, error) {
	var out dataScheduleEnvelope
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/data_schedules/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.DataSchedule, nil
}

func (c *Client) UpdateDataSchedule(ctx context.Context, id int, input DataSchedule) (*DataSchedule, error) {
	body := dataScheduleEnvelope{DataSchedule: input}
	var out dataScheduleEnvelope
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/data_schedules/%d", id), nil, body, &out); err != nil {
		return nil, err
	}
	return &out.DataSchedule, nil
}

func (c *Client) DeleteDataSchedule(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/data_schedules/%d", id), nil, nil, nil)
}
