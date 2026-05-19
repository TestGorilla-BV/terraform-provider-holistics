package client

import (
	"context"
	"fmt"
	"net/http"
)

type DataSourceSettings struct {
	RequireSSL       *bool   `json:"require_ssl,omitempty"`
	QueryTimeout     *int    `json:"query_timeout,omitempty"`
	EnableSchemaInfo *bool   `json:"enable_schema_info,omitempty"`
	UseConnectionStr *bool   `json:"use_connection_str,omitempty"`
	Timezone         *string `json:"timezone,omitempty"`
}

type DataSource struct {
	ID        int                 `json:"id"`
	Name      string              `json:"name"`
	DBType    string              `json:"dbtype"`
	Settings  *DataSourceSettings `json:"settings,omitempty"`
	IsSample  *bool               `json:"is_sample,omitempty"`
	IsDefault *bool               `json:"is_default,omitempty"`
}

func (c *Client) GetDataSource(ctx context.Context, id int) (*DataSource, error) {
	var out struct {
		DataSource DataSource `json:"data_source"`
	}
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/data_sources/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.DataSource, nil
}
