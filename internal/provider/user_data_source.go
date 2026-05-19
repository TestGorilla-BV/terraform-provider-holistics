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
	_ datasource.DataSource              = (*userDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userDataSource)(nil)
)

type userDataSource struct {
	client *client.Client
}

type userDataModel struct {
	ID                       types.Int64  `tfsdk:"id"`
	Email                    types.String `tfsdk:"email"`
	Name                     types.String `tfsdk:"name"`
	Initials                 types.String `tfsdk:"initials"`
	Role                     types.String `tfsdk:"role"`
	IsDeleted                types.Bool   `tfsdk:"is_deleted"`
	IsActivated              types.Bool   `tfsdk:"is_activated"`
	HasAuthenticationToken   types.Bool   `tfsdk:"has_authentication_token"`
	AllowAuthenticationToken types.Bool   `tfsdk:"allow_authentication_token"`
	EnableExportData         types.Bool   `tfsdk:"enable_export_data"`
	CurrentSignInAt          types.String `tfsdk:"current_sign_in_at"`
	LastSignInAt             types.String `tfsdk:"last_sign_in_at"`
	CreatedAt                types.String `tfsdk:"created_at"`
	Title                    types.String `tfsdk:"title"`
	JobTitle                 types.String `tfsdk:"job_title"`
	GroupIDs                 types.Set    `tfsdk:"group_ids"`
}

func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a single Holistics user by ID.",
		Attributes:  userDataSourceAttributes(),
	}
}

func userDataSourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":                         schema.Int64Attribute{Required: true, Description: "User ID."},
		"email":                      schema.StringAttribute{Computed: true},
		"name":                       schema.StringAttribute{Computed: true},
		"initials":                   schema.StringAttribute{Computed: true},
		"role":                       schema.StringAttribute{Computed: true},
		"is_deleted":                 schema.BoolAttribute{Computed: true},
		"is_activated":               schema.BoolAttribute{Computed: true},
		"has_authentication_token":   schema.BoolAttribute{Computed: true},
		"allow_authentication_token": schema.BoolAttribute{Computed: true},
		"enable_export_data":         schema.BoolAttribute{Computed: true},
		"current_sign_in_at":         schema.StringAttribute{Computed: true},
		"last_sign_in_at":            schema.StringAttribute{Computed: true},
		"created_at":                 schema.StringAttribute{Computed: true},
		"title":                      schema.StringAttribute{Computed: true},
		"job_title":                  schema.StringAttribute{Computed: true},
		"group_ids":                  schema.SetAttribute{Computed: true, ElementType: types.Int64Type},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input userDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	u, err := d.client.GetUser(ctx, int(input.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read user", err.Error())
		return
	}
	m, diags := userToModel(ctx, u)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func userToModel(ctx context.Context, u *client.User) (userDataModel, diag.Diagnostics) {
	groups, diags := intSliceToSet(ctx, u.GroupIDs)
	return userDataModel{
		ID:                       types.Int64Value(int64(u.ID)),
		Email:                    types.StringValue(u.Email),
		Name:                     stringFromPtr(u.Name),
		Initials:                 types.StringValue(u.Initials),
		Role:                     types.StringValue(u.Role),
		IsDeleted:                types.BoolValue(u.IsDeleted),
		IsActivated:              types.BoolValue(u.IsActivated),
		HasAuthenticationToken:   types.BoolValue(u.HasAuthenticationToken),
		AllowAuthenticationToken: types.BoolValue(u.AllowAuthenticationToken),
		EnableExportData:         types.BoolValue(u.EnableExportData),
		CurrentSignInAt:          stringFromPtr(u.CurrentSignInAt),
		LastSignInAt:             stringFromPtr(u.LastSignInAt),
		CreatedAt:                types.StringValue(u.CreatedAt),
		Title:                    stringFromPtr(u.Title),
		JobTitle:                 stringFromPtr(u.JobTitle),
		GroupIDs:                 groups,
	}, diags
}
