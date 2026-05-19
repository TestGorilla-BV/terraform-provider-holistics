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
	_ datasource.DataSource              = (*dashboardDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dashboardDataSource)(nil)
)

type dashboardDataSource struct {
	client *client.Client
}

type dashboardDataModel struct {
	ID         types.Int64  `tfsdk:"id"`
	OwnerID    types.Int64  `tfsdk:"owner_id"`
	Title      types.String `tfsdk:"title"`
	CategoryID types.Int64  `tfsdk:"category_id"`
	Version    types.Int64  `tfsdk:"version"`
	URL        types.String `tfsdk:"url"`
	Tags       types.List   `tfsdk:"tags"`
}

func NewDashboardDataSource() datasource.DataSource {
	return &dashboardDataSource{}
}

func (d *dashboardDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (d *dashboardDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Holistics dashboard by ID.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.Int64Attribute{Required: true},
			"owner_id":    schema.Int64Attribute{Computed: true},
			"title":       schema.StringAttribute{Computed: true},
			"category_id": schema.Int64Attribute{Computed: true},
			"version":     schema.Int64Attribute{Computed: true},
			"url":         schema.StringAttribute{Computed: true},
			"tags":        schema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
	}
}

func (d *dashboardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *dashboardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input dashboardDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dash, err := d.client.GetDashboard(ctx, int(input.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard", err.Error())
		return
	}
	tags, diags := stringSliceToList(ctx, dash.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	m := dashboardDataModel{
		ID:    types.Int64Value(int64(dash.ID)),
		Title: types.StringValue(dash.Title),
		Tags:  tags,
	}
	if dash.OwnerID != nil {
		m.OwnerID = types.Int64Value(int64(*dash.OwnerID))
	} else {
		m.OwnerID = types.Int64Null()
	}
	if dash.CategoryID != nil {
		m.CategoryID = types.Int64Value(int64(*dash.CategoryID))
	} else {
		m.CategoryID = types.Int64Null()
	}
	if dash.Version != nil {
		m.Version = types.Int64Value(int64(*dash.Version))
	} else {
		m.Version = types.Int64Null()
	}
	m.URL = stringFromPtr(dash.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
