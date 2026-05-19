package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ resource.Resource                = (*shareableLinkResource)(nil)
	_ resource.ResourceWithConfigure   = (*shareableLinkResource)(nil)
	_ resource.ResourceWithImportState = (*shareableLinkResource)(nil)
)

type shareableLinkResource struct {
	client *client.Client
}

type rowBasedPermissionRuleModel struct {
	Condition *conditionModel `tfsdk:"condition"`
	FieldPath *fieldPathModel `tfsdk:"field_path"`
}

type permissionRulesModel struct {
	RowBased []rowBasedPermissionRuleModel `tfsdk:"row_based"`
}

type shareableLinkResourceModel struct {
	ID                   types.String               `tfsdk:"id"`
	ResourceType         types.String               `tfsdk:"resource_type"`
	ResourceID           types.Int64                `tfsdk:"resource_id"`
	Title                types.String               `tfsdk:"title"`
	PasswordEnabled      types.Bool                 `tfsdk:"password_enabled"`
	Password             types.String               `tfsdk:"password"`
	ExpiredAt            types.String               `tfsdk:"expired_at"`
	DynamicFilterPresets []dynamicFilterPresetModel `tfsdk:"dynamic_filter_presets"`
	PermissionRules      *permissionRulesModel      `tfsdk:"permission_rules"`
}

func NewShareableLinkResource() resource.Resource {
	return &shareableLinkResource{}
}

func (r *shareableLinkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shareable_link"
}

func (r *shareableLinkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A public shareable link to a Holistics Dashboard.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Shareable link ID.",
				PlanModifiers: []planmodifier.String{stringplanUseStateForUnknown()},
			},
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "Resource type to share. Currently only `Dashboard`.",
				Validators:  []validator.String{stringvalidator.OneOf("Dashboard")},
			},
			"resource_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the resource to share.",
			},
			"title":            schema.StringAttribute{Optional: true},
			"password_enabled": schema.BoolAttribute{Optional: true, Computed: true},
			"password": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				Description:   "Password to access the link (only set when password_enabled is true).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expired_at": schema.StringAttribute{Optional: true},
			"dynamic_filter_presets": schema.ListNestedAttribute{
				Optional:     true,
				NestedObject: dynamicFilterPresetNested(),
			},
			"permission_rules": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"row_based": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"field_path": schema.SingleNestedAttribute{
									Required: true,
									Attributes: map[string]schema.Attribute{
										"field_name":  schema.StringAttribute{Required: true},
										"model_id":    schema.StringAttribute{Optional: true},
										"data_set_id": schema.Int64Attribute{Optional: true},
										"is_metric":   schema.BoolAttribute{Optional: true},
									},
								},
								"condition": schema.SingleNestedAttribute{
									Required: true,
									Attributes: map[string]schema.Attribute{
										"operator": schema.StringAttribute{Required: true},
										"modifier": schema.StringAttribute{Optional: true},
										"values":   schema.ListAttribute{ElementType: types.StringType, Optional: true},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *shareableLinkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *shareableLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan shareableLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	input, diags := buildShareableLinkInput(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	sl, err := r.client.CreateShareableLink(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create shareable link", err.Error())
		return
	}
	resp.Diagnostics.Append(writeShareableLinkState(ctx, sl, plan.Password, &resp.State)...)
}

func (r *shareableLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state shareableLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sl, err := r.client.GetShareableLink(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read shareable link", err.Error())
		return
	}
	resp.Diagnostics.Append(writeShareableLinkState(ctx, sl, state.Password, &resp.State)...)
}

func (r *shareableLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state shareableLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	input, diags := buildShareableLinkInput(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	sl, err := r.client.UpdateShareableLink(ctx, state.ID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update shareable link", err.Error())
		return
	}
	resp.Diagnostics.Append(writeShareableLinkState(ctx, sl, plan.Password, &resp.State)...)
}

func (r *shareableLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state shareableLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteShareableLink(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete shareable link", err.Error())
	}
}

func (r *shareableLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func buildShareableLinkInput(ctx context.Context, m *shareableLinkResourceModel) (client.ShareableLink, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := client.ShareableLink{
		ResourceType:    m.ResourceType.ValueString(),
		ResourceID:      json.Number(strconv.FormatInt(m.ResourceID.ValueInt64(), 10)),
		Title:           stringPtrFromTF(m.Title),
		PasswordEnabled: boolPtrFromTF(m.PasswordEnabled),
		Password:        stringPtrFromTF(m.Password),
		ExpiredAt:       stringPtrFromTF(m.ExpiredAt),
	}
	presets, d := dynamicFilterPresetsToAPI(ctx, m.DynamicFilterPresets)
	diags.Append(d...)
	out.DynamicFilterPresets = presets

	if m.PermissionRules != nil {
		pr := &client.PermissionRules{}
		for _, rule := range m.PermissionRules.RowBased {
			r := client.RowBasedPermissionRule{}
			if rule.FieldPath != nil {
				r.FieldPath = client.FieldPath{
					FieldName: rule.FieldPath.FieldName.ValueString(),
					ModelID:   json.Number(rule.FieldPath.ModelID.ValueString()),
					IsMetric:  boolPtrFromTF(rule.FieldPath.IsMetric),
				}
				if !rule.FieldPath.DataSetID.IsNull() && !rule.FieldPath.DataSetID.IsUnknown() {
					v := int(rule.FieldPath.DataSetID.ValueInt64())
					r.FieldPath.DataSetID = &v
				}
			}
			if rule.Condition != nil {
				values, d := listToStringSlice(ctx, rule.Condition.Values)
				diags.Append(d...)
				cv := make([]client.ConditionValue, len(values))
				for i, v := range values {
					cv[i] = client.ConditionValueFromString(v)
				}
				r.Condition = client.Condition{
					Operator: rule.Condition.Operator.ValueString(),
					Modifier: stringPtrFromTF(rule.Condition.Modifier),
					Values:   cv,
				}
			}
			pr.RowBased = append(pr.RowBased, r)
		}
		out.PermissionRules = pr
	}
	return out, diags
}

// writeShareableLinkState writes the API response to state. The API doesn't
// echo the password back, so we preserve the user-supplied value.
func writeShareableLinkState(ctx context.Context, sl *client.ShareableLink, password types.String, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	if sl.ID == "" {
		diags.AddError("Invalid shareable_link response", "id missing")
		return diags
	}
	rid, err := strconv.ParseInt(string(sl.ResourceID), 10, 64)
	if err != nil {
		diags.AddError("Invalid resource_id in response", err.Error())
		return diags
	}
	m := shareableLinkResourceModel{
		ID:              types.StringValue(string(sl.ID)),
		ResourceType:    types.StringValue(sl.ResourceType),
		ResourceID:      types.Int64Value(rid),
		Title:           stringFromPtr(sl.Title),
		PasswordEnabled: boolFromPtr(sl.PasswordEnabled),
		Password:        password,
		ExpiredAt:       stringFromPtr(sl.ExpiredAt),
	}

	presets, d := dynamicFilterPresetsFromAPI(ctx, sl.DynamicFilterPresets)
	diags.Append(d...)
	m.DynamicFilterPresets = presets

	if sl.PermissionRules != nil && len(sl.PermissionRules.RowBased) > 0 {
		pr := &permissionRulesModel{}
		for _, rule := range sl.PermissionRules.RowBased {
			vals := make([]string, 0, len(rule.Condition.Values))
			for _, v := range rule.Condition.Values {
				switch {
				case v.String != nil:
					vals = append(vals, *v.String)
				case v.Number != nil:
					vals = append(vals, strconv.FormatFloat(*v.Number, 'f', -1, 64))
				case v.Bool != nil:
					vals = append(vals, strconv.FormatBool(*v.Bool))
				}
			}
			valsList, d := stringSliceToList(ctx, vals)
			diags.Append(d...)
			fp := &fieldPathModel{
				FieldName: types.StringValue(rule.FieldPath.FieldName),
				ModelID:   types.StringValue(string(rule.FieldPath.ModelID)),
				IsMetric:  boolFromPtr(rule.FieldPath.IsMetric),
			}
			if rule.FieldPath.DataSetID != nil {
				fp.DataSetID = types.Int64Value(int64(*rule.FieldPath.DataSetID))
			} else {
				fp.DataSetID = types.Int64Null()
			}
			pr.RowBased = append(pr.RowBased, rowBasedPermissionRuleModel{
				FieldPath: fp,
				Condition: &conditionModel{
					Operator: types.StringValue(rule.Condition.Operator),
					Modifier: stringFromPtr(rule.Condition.Modifier),
					Values:   valsList,
				},
			})
		}
		m.PermissionRules = pr
	}

	diags.Append(state.Set(ctx, &m)...)
	return diags
}
