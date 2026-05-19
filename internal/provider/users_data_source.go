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
	_ datasource.DataSource              = (*usersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*usersDataSource)(nil)
)

type usersDataSource struct {
	client *client.Client
}

type usersDataModel struct {
	SearchTerm             types.String    `tfsdk:"search_term"`
	Role                   types.String    `tfsdk:"role"`
	Statuses               types.Set       `tfsdk:"statuses"`
	IDs                    types.Set       `tfsdk:"ids"`
	HasAuthenticationToken types.Bool      `tfsdk:"has_authentication_token"`
	ExcludeDeleted         types.Bool      `tfsdk:"exclude_deleted"`
	Users                  []userDataModel `tfsdk:"users"`
}

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

func (d *usersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List Holistics users, optionally filtered.",
		Attributes: map[string]schema.Attribute{
			"search_term":              schema.StringAttribute{Optional: true, Description: "Search by email, name, initials, or group name."},
			"role":                     schema.StringAttribute{Optional: true, Description: "Filter by role."},
			"statuses":                 schema.SetAttribute{Optional: true, ElementType: types.StringType, Description: "Filter by user statuses."},
			"ids":                      schema.SetAttribute{Optional: true, ElementType: types.Int64Type, Description: "Filter by user IDs."},
			"has_authentication_token": schema.BoolAttribute{Optional: true},
			"exclude_deleted":          schema.BoolAttribute{Optional: true},
			"users": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: userListItemAttributes(),
				},
			},
		},
	}
}

// userListItemAttributes returns the same set as userDataSourceAttributes but
// with `id` as Computed (since list items aren't queried by id).
func userListItemAttributes() map[string]schema.Attribute {
	attrs := userDataSourceAttributes()
	attrs["id"] = schema.Int64Attribute{Computed: true}
	return attrs
}

func (d *usersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input usersDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filter := client.ListUsersFilter{
		SearchTerm:             input.SearchTerm.ValueString(),
		Role:                   input.Role.ValueString(),
		HasAuthenticationToken: boolPtrFromTF(input.HasAuthenticationToken),
		ExcludeDeleted:         boolPtrFromTF(input.ExcludeDeleted),
	}
	if statuses, diags := setToStringSlice(ctx, input.Statuses); !diags.HasError() {
		filter.Statuses = statuses
	} else {
		resp.Diagnostics.Append(diags...)
		return
	}
	if ids, diags := setToIntSlice(ctx, input.IDs); !diags.HasError() {
		filter.IDs = ids
	} else {
		resp.Diagnostics.Append(diags...)
		return
	}

	users, err := d.client.ListUsers(ctx, filter)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list users", err.Error())
		return
	}

	out := make([]userDataModel, len(users))
	for i := range users {
		m, diags := userToModel(ctx, &users[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		out[i] = m
	}
	input.Users = out
	resp.Diagnostics.Append(resp.State.Set(ctx, &input)...)
}
