package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ datasource.DataSource              = (*groupsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*groupsDataSource)(nil)
)

type groupsDataSource struct {
	client *client.Client
}

type groupItemModel struct {
	ID      types.Int64 `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	UserIDs types.Set    `tfsdk:"user_ids"`
}

type groupsDataModel struct {
	Groups []groupItemModel `tfsdk:"groups"`
}

func NewGroupsDataSource() datasource.DataSource {
	return &groupsDataSource{}
}

func (d *groupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_groups"
}

func (d *groupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all Holistics groups with their member user IDs.",
		Attributes: map[string]schema.Attribute{
			"groups": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.Int64Attribute{Computed: true, Description: "Group ID."},
						"name": schema.StringAttribute{Computed: true, Description: "Group name."},
						"user_ids": schema.SetAttribute{
							Computed:    true,
							ElementType: types.Int64Type,
							Description: "User IDs that belong to this group.",
						},
					},
				},
			},
		},
	}
}

func (d *groupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	groups, err := d.client.ListGroups(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list groups", err.Error())
		return
	}
	out := make([]groupItemModel, 0, len(groups))
	for i := range groups {
		m, diags := groupToModel(ctx, &groups[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		out = append(out, m)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &groupsDataModel{Groups: out})...)
}

func groupToModel(ctx context.Context, g *client.Group) (groupItemModel, diag.Diagnostics) {
	userIDs, diags := intSliceToSet(ctx, g.UserIDs)
	m := groupItemModel{
		Name:    types.StringValue(g.Name),
		UserIDs: userIDs,
	}
	if g.ID != nil {
		m.ID = types.Int64Value(int64(*g.ID))
	} else {
		m.ID = types.Int64Null()
	}
	return m, diags
}
