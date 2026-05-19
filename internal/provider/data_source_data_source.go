package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ datasource.DataSource              = (*dataSourceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dataSourceDataSource)(nil)
)

type dataSourceDataSource struct {
	client *client.Client
}

type dataSourceSettingsModel struct {
	RequireSSL       types.Bool   `tfsdk:"require_ssl"`
	QueryTimeout     types.Int64  `tfsdk:"query_timeout"`
	EnableSchemaInfo types.Bool   `tfsdk:"enable_schema_info"`
	UseConnectionStr types.Bool   `tfsdk:"use_connection_str"`
	Timezone         types.String `tfsdk:"timezone"`
}

type dataSourceDataModel struct {
	ID        types.Int64              `tfsdk:"id"`
	Name      types.String             `tfsdk:"name"`
	DBType    types.String             `tfsdk:"dbtype"`
	Settings  *dataSourceSettingsModel `tfsdk:"settings"`
	IsSample  types.Bool               `tfsdk:"is_sample"`
	IsDefault types.Bool               `tfsdk:"is_default"`
}

func NewDataSourceDataSource() datasource.DataSource {
	return &dataSourceDataSource{}
}

func (d *dataSourceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_source"
}

func (d *dataSourceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Holistics data source (database connection) by ID.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Required: true},
			"name":       schema.StringAttribute{Computed: true},
			"dbtype":     schema.StringAttribute{Computed: true},
			"is_sample":  schema.BoolAttribute{Computed: true},
			"is_default": schema.BoolAttribute{Computed: true},
			"settings": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"require_ssl":        schema.BoolAttribute{Computed: true},
					"query_timeout":      schema.Int64Attribute{Computed: true},
					"enable_schema_info": schema.BoolAttribute{Computed: true},
					"use_connection_str": schema.BoolAttribute{Computed: true},
					"timezone":           schema.StringAttribute{Computed: true},
				},
			},
		},
	}
	_ = types.StringType
}

func (d *dataSourceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *dataSourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input dataSourceDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds, err := d.client.GetDataSource(ctx, int(input.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read data source", err.Error())
		return
	}
	m := dataSourceDataModel{
		ID:        types.Int64Value(int64(ds.ID)),
		Name:      types.StringValue(ds.Name),
		DBType:    types.StringValue(ds.DBType),
		IsSample:  boolFromPtr(ds.IsSample),
		IsDefault: boolFromPtr(ds.IsDefault),
	}
	if ds.Settings != nil {
		s := &dataSourceSettingsModel{
			RequireSSL:       boolFromPtr(ds.Settings.RequireSSL),
			EnableSchemaInfo: boolFromPtr(ds.Settings.EnableSchemaInfo),
			UseConnectionStr: boolFromPtr(ds.Settings.UseConnectionStr),
			Timezone:         stringFromPtr(ds.Settings.Timezone),
		}
		if ds.Settings.QueryTimeout != nil {
			s.QueryTimeout = types.Int64Value(int64(*ds.Settings.QueryTimeout))
		} else {
			s.QueryTimeout = types.Int64Null()
		}
		m.Settings = s
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
