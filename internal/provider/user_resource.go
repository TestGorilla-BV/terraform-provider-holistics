package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ resource.Resource                = (*userResource)(nil)
	_ resource.ResourceWithConfigure   = (*userResource)(nil)
	_ resource.ResourceWithImportState = (*userResource)(nil)
)

type userResource struct {
	client *client.Client
}

type userResourceModel struct {
	ID                       types.Int64  `tfsdk:"id"`
	Email                    types.String `tfsdk:"email"`
	Role                     types.String `tfsdk:"role"`
	Name                     types.String `tfsdk:"name"`
	Title                    types.String `tfsdk:"title"`
	JobTitle                 types.String `tfsdk:"job_title"`
	AllowAuthenticationToken types.Bool   `tfsdk:"allow_authentication_token"`
	EnableExportData         types.Bool   `tfsdk:"enable_export_data"`
	GroupIDs                 types.Set    `tfsdk:"group_ids"`
	InviteMessage            types.String `tfsdk:"invite_message"`

	Initials               types.String `tfsdk:"initials"`
	IsActivated            types.Bool   `tfsdk:"is_activated"`
	HasAuthenticationToken types.Bool   `tfsdk:"has_authentication_token"`
	CurrentSignInAt        types.String `tfsdk:"current_sign_in_at"`
	LastSignInAt           types.String `tfsdk:"last_sign_in_at"`
	CreatedAt              types.String `tfsdk:"created_at"`
}

func NewUserResource() resource.Resource {
	return &userResource{}
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Holistics user. Created via `/users/invite`; deletion is a soft-delete (`is_deleted=true`). " +
			"If a Terraform-managed user is destroyed and then re-created with the same email, the provider transparently " +
			"restores the soft-deleted record instead of failing with `email already in use`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:      true,
				Description:   "User ID.",
				PlanModifiers: []planmodifier.Int64{int64planUseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "User email address. Used as the invitation target and the import key. Changing this forces resource replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required:    true,
				Description: "Holistics role (e.g. `admin`, `analyst`, `user`, `explorer`, `embed`).",
			},
			"name":      schema.StringAttribute{Optional: true, Computed: true, Description: "Display name. Set after invite via update."},
			"title":     schema.StringAttribute{Optional: true, Computed: true},
			"job_title": schema.StringAttribute{Optional: true, Computed: true},
			"allow_authentication_token": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the user is allowed to mint API keys. Setting this to `false` after a token exists has the same effect as revoking it.",
			},
			"enable_export_data": schema.BoolAttribute{Optional: true, Computed: true, Description: "Enterprise: allow this user to export data."},
			"group_ids": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Groups the user is a member of. Replaces the full membership list on update.",
			},
			"invite_message": schema.StringAttribute{
				Optional:    true,
				Description: "Greeting sent in the invitation email. Used only at create time; changes do not trigger updates.",
			},

			"initials":                 schema.StringAttribute{Computed: true},
			"is_activated":             schema.BoolAttribute{Computed: true},
			"has_authentication_token": schema.BoolAttribute{Computed: true},
			"current_sign_in_at":       schema.StringAttribute{Computed: true},
			"last_sign_in_at":          schema.StringAttribute{Computed: true},
			"created_at":               schema.StringAttribute{Computed: true},
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupIDs, diags := setToIntSlice(ctx, plan.GroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If a soft-deleted user with this email already exists, restore it
	// rather than failing the invite. This keeps `destroy → apply` cycles
	// working transparently.
	existing, err := r.client.GetUserByEmail(ctx, plan.Email.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to check for existing user", err.Error())
		return
	}

	var u *client.User
	if existing != nil && existing.IsDeleted {
		if err := r.client.RestoreUser(ctx, existing.ID); err != nil {
			resp.Diagnostics.AddError("Failed to restore soft-deleted user", err.Error())
			return
		}
		u = existing
	} else if existing != nil && !existing.IsDeleted {
		resp.Diagnostics.AddError(
			"User already exists",
			fmt.Sprintf("a user with email %q already exists (id=%d) and is not soft-deleted. Import it with `terraform import` instead of creating.", plan.Email.ValueString(), existing.ID),
		)
		return
	} else {
		invited, err := r.client.InviteUser(ctx, client.InviteUserInput{
			Email:                    plan.Email.ValueString(),
			Role:                     plan.Role.ValueString(),
			AllowAuthenticationToken: boolPtrFromTF(plan.AllowAuthenticationToken),
			EnableExportData:         boolPtrFromTF(plan.EnableExportData),
			GroupIDs:                 groupIDs,
			Message:                  stringPtrFromTF(plan.InviteMessage),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to invite user", err.Error())
			return
		}
		u = invited
	}

	// After invite/restore, send a follow-up PUT for fields that aren't
	// settable at invite time (name/title/job_title) or that may need to
	// be applied to a restored user.
	update := client.UpdateUserInput{
		Name:                     stringPtrFromTF(plan.Name),
		Title:                    stringPtrFromTF(plan.Title),
		JobTitle:                 stringPtrFromTF(plan.JobTitle),
		AllowAuthenticationToken: boolPtrFromTF(plan.AllowAuthenticationToken),
		EnableExportData:         boolPtrFromTF(plan.EnableExportData),
	}
	if !plan.Role.IsNull() && !plan.Role.IsUnknown() {
		v := plan.Role.ValueString()
		update.Role = &v
	}
	if !plan.GroupIDs.IsNull() && !plan.GroupIDs.IsUnknown() {
		update.GroupIDs = &groupIDs
	}
	if anyUpdateFieldSet(update) {
		updated, err := r.client.UpdateUser(ctx, u.ID, update)
		if err != nil {
			resp.Diagnostics.AddError("Failed to apply post-invite update", err.Error())
			return
		}
		u = updated
	}

	resp.Diagnostics.Append(writeUserState(ctx, u, plan.InviteMessage, &resp.State)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	u, err := r.client.GetUser(ctx, int(state.ID.ValueInt64()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read user", err.Error())
		return
	}
	if u.IsDeleted {
		// Treat soft-deleted as "gone" so a subsequent apply will re-create
		// (which transparently restores via the create path above).
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(writeUserState(ctx, u, state.InviteMessage, &resp.State)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := int(state.ID.ValueInt64())

	groupIDs, diags := setToIntSlice(ctx, plan.GroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := plan.Role.ValueString()
	update := client.UpdateUserInput{
		Name:                     stringPtrFromTF(plan.Name),
		Title:                    stringPtrFromTF(plan.Title),
		JobTitle:                 stringPtrFromTF(plan.JobTitle),
		Role:                     &role,
		AllowAuthenticationToken: boolPtrFromTF(plan.AllowAuthenticationToken),
		EnableExportData:         boolPtrFromTF(plan.EnableExportData),
		GroupIDs:                 &groupIDs,
	}

	u, err := r.client.UpdateUser(ctx, id, update)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update user", err.Error())
		return
	}
	resp.Diagnostics.Append(writeUserState(ctx, u, plan.InviteMessage, &resp.State)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.SoftDeleteUser(ctx, int(state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to soft-delete user", err.Error())
	}
}

// ImportState accepts either an integer user ID or an email address. Email
// lookup is the friendlier path since the Holistics UI doesn't surface user
// IDs prominently; the provider resolves the email to the underlying ID.
func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if id, err := strconv.ParseInt(req.ID, 10, 64); err == nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
		return
	}
	if !strings.Contains(req.ID, "@") {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("expected an integer user ID or an email address, got %q", req.ID),
		)
		return
	}
	u, err := r.client.GetUserByEmail(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to look up user by email", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), int64(u.ID))...)
}

func anyUpdateFieldSet(u client.UpdateUserInput) bool {
	return u.Name != nil || u.Title != nil || u.JobTitle != nil || u.Role != nil ||
		u.AllowAuthenticationToken != nil || u.EnableExportData != nil || u.GroupIDs != nil
}

// writeUserState copies the API user into state. inviteMessage is preserved
// from the plan/prior state since the API never echoes it back.
func writeUserState(ctx context.Context, u *client.User, inviteMessage types.String, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	groups, d := intSliceToSet(ctx, u.GroupIDs)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	m := userResourceModel{
		ID:                       types.Int64Value(int64(u.ID)),
		Email:                    types.StringValue(u.Email),
		Role:                     types.StringValue(u.Role),
		Name:                     stringFromPtr(u.Name),
		Title:                    stringFromPtr(u.Title),
		JobTitle:                 stringFromPtr(u.JobTitle),
		AllowAuthenticationToken: types.BoolValue(u.AllowAuthenticationToken),
		EnableExportData:         types.BoolValue(u.EnableExportData),
		GroupIDs:                 groups,
		InviteMessage:            inviteMessage,

		Initials:               types.StringValue(u.Initials),
		IsActivated:            types.BoolValue(u.IsActivated),
		HasAuthenticationToken: types.BoolValue(u.HasAuthenticationToken),
		CurrentSignInAt:        stringFromPtr(u.CurrentSignInAt),
		LastSignInAt:           stringFromPtr(u.LastSignInAt),
		CreatedAt:              types.StringValue(u.CreatedAt),
	}
	diags.Append(state.Set(ctx, &m)...)
	return diags
}
