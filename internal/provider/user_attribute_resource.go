package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ resource.Resource                = (*userAttributeResource)(nil)
	_ resource.ResourceWithConfigure   = (*userAttributeResource)(nil)
	_ resource.ResourceWithImportState = (*userAttributeResource)(nil)
)

type userAttributeResource struct {
	client *client.Client
}

type userAttributeResourceModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	AttributeType     types.String `tfsdk:"attribute_type"`
	Label             types.String `tfsdk:"label"`
	Description       types.String `tfsdk:"description"`
	IsSystemAttribute types.Bool   `tfsdk:"is_system_attribute"`
}

func NewUserAttributeResource() resource.Resource {
	return &userAttributeResource{}
}

func (r *userAttributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute"
}

func (r *userAttributeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Holistics user attribute (custom or system).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "User attribute ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planUseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Internal name of the attribute.",
				Required:    true,
			},
			"attribute_type": schema.StringAttribute{
				Description: "Attribute data type (e.g. `string`, `number`, `boolean`).",
				Required:    true,
			},
			"label": schema.StringAttribute{
				Description: "Display label.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Optional description.",
				Optional:    true,
			},
			"is_system_attribute": schema.BoolAttribute{
				Description: "True for built-in system attributes.",
				Computed:    true,
			},
		},
	}
}

func (r *userAttributeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *userAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.UserAttributeInput{
		Name:          plan.Name.ValueString(),
		AttributeType: plan.AttributeType.ValueString(),
		Label:         plan.Label.ValueString(),
		Description:   stringPtrFromTF(plan.Description),
	}
	ua, err := r.client.CreateUserAttribute(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create user attribute", err.Error())
		return
	}
	resp.Diagnostics.Append(writeUserAttributeState(ctx, ua, &resp.State)...)
}

func (r *userAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ua, err := r.client.GetUserAttribute(ctx, int(state.ID.ValueInt64()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read user attribute", err.Error())
		return
	}
	resp.Diagnostics.Append(writeUserAttributeState(ctx, ua, &resp.State)...)
}

func (r *userAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userAttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.UserAttributeInput{
		Name:          plan.Name.ValueString(),
		AttributeType: plan.AttributeType.ValueString(),
		Label:         plan.Label.ValueString(),
		Description:   stringPtrFromTF(plan.Description),
	}
	ua, err := r.client.UpdateUserAttribute(ctx, int(state.ID.ValueInt64()), input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update user attribute", err.Error())
		return
	}
	resp.Diagnostics.Append(writeUserAttributeState(ctx, ua, &resp.State)...)
}

func (r *userAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteUserAttribute(ctx, int(state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete user attribute", err.Error())
	}
}

func (r *userAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected integer user attribute ID, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func writeUserAttributeState(ctx context.Context, ua *client.UserAttribute, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	if ua.ID == nil {
		diags.AddError("Invalid user_attribute response", "id missing")
		return diags
	}
	m := userAttributeResourceModel{
		ID:                types.Int64Value(int64(*ua.ID)),
		Name:              types.StringValue(ua.Name),
		AttributeType:     types.StringValue(ua.AttributeType),
		Label:             types.StringValue(ua.Label),
		Description:       stringFromPtr(ua.Description),
		IsSystemAttribute: boolFromPtr(ua.IsSystemAttribute),
	}
	diags.Append(state.Set(ctx, &m)...)
	return diags
}
