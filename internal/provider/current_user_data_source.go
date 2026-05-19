package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ datasource.DataSource              = (*currentUserDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*currentUserDataSource)(nil)
)

type currentUserDataSource struct {
	client *client.Client
}

func NewCurrentUserDataSource() datasource.DataSource {
	return &currentUserDataSource{}
}

func (d *currentUserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_current_user"
}

func (d *currentUserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := userDataSourceAttributes()
	attrs["id"] = schema.Int64Attribute{Computed: true}
	resp.Schema = schema.Schema{
		Description: "The Holistics user that owns the configured API key.",
		Attributes:  attrs,
	}
}

func (d *currentUserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *currentUserDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	u, err := d.client.GetCurrentUser(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read current user", err.Error())
		return
	}
	m, diags := userToModel(ctx, u)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
