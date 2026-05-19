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
	_ resource.Resource                = (*groupResource)(nil)
	_ resource.ResourceWithConfigure   = (*groupResource)(nil)
	_ resource.ResourceWithImportState = (*groupResource)(nil)
)

type groupResource struct {
	client *client.Client
}

type groupResourceModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	UserIDs types.Set    `tfsdk:"user_ids"`
}

func NewGroupResource() resource.Resource {
	return &groupResource{}
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Holistics group. User membership is managed via `user_ids`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "Group ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planUseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Group name.",
				Required:    true,
			},
			"user_ids": schema.SetAttribute{
				Description:   "User IDs that are members of this group.",
				Optional:      true,
				Computed:      true,
				ElementType:   types.Int64Type,
				PlanModifiers: []planmodifier.Set{setplanUseStateForUnknown()},
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.CreateGroup(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}
	if g.ID == nil {
		resp.Diagnostics.AddError("Failed to create group", "API response missing group id")
		return
	}

	desired, diags := setToIntSlice(ctx, plan.UserIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, uid := range desired {
		if err := r.client.AddGroupUser(ctx, *g.ID, uid); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Failed to add user %d to group %d", uid, *g.ID), err.Error())
			return
		}
	}

	fresh, err := r.client.GetGroupWithUsers(ctx, *g.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to refresh group after create", err.Error())
		return
	}
	resp.Diagnostics.Append(writeGroupState(ctx, fresh, &resp.State)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.GetGroupWithUsers(ctx, int(state.ID.ValueInt64()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}
	resp.Diagnostics.Append(writeGroupState(ctx, g, &resp.State)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := int(state.ID.ValueInt64())

	if plan.Name.ValueString() != state.Name.ValueString() {
		if _, err := r.client.UpdateGroup(ctx, id, plan.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("Failed to update group", err.Error())
			return
		}
	}

	desired, diags := setToIntSlice(ctx, plan.UserIDs)
	resp.Diagnostics.Append(diags...)
	current, diags := setToIntSlice(ctx, state.UserIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	toAdd, toRemove := diffIntSlices(desired, current)
	for _, uid := range toAdd {
		if err := r.client.AddGroupUser(ctx, id, uid); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Failed to add user %d to group %d", uid, id), err.Error())
			return
		}
	}
	for _, uid := range toRemove {
		if err := r.client.RemoveGroupUser(ctx, id, uid); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Failed to remove user %d from group %d", uid, id), err.Error())
			return
		}
	}

	fresh, err := r.client.GetGroupWithUsers(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Failed to refresh group after update", err.Error())
		return
	}
	resp.Diagnostics.Append(writeGroupState(ctx, fresh, &resp.State)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteGroup(ctx, int(state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected integer group ID, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func writeGroupState(ctx context.Context, g *client.Group, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	if g.ID == nil {
		diags.AddError("Invalid group response", "group id missing")
		return diags
	}
	userIDs, d := intSliceToSet(ctx, g.UserIDs)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	m := groupResourceModel{
		ID:      types.Int64Value(int64(*g.ID)),
		Name:    types.StringValue(g.Name),
		UserIDs: userIDs,
	}
	diags.Append(state.Set(ctx, &m)...)
	return diags
}
