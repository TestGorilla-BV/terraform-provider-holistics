package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ resource.Resource                = (*dataAlertResource)(nil)
	_ resource.ResourceWithConfigure   = (*dataAlertResource)(nil)
	_ resource.ResourceWithImportState = (*dataAlertResource)(nil)
)

type dataAlertResource struct {
	client *client.Client
}

type alertEmailOptionsModel struct {
	BodyText types.String `tfsdk:"body_text"`
}

type alertEmailDestModel struct {
	Title      types.String            `tfsdk:"title"`
	Recipients types.List              `tfsdk:"recipients"`
	Options    *alertEmailOptionsModel `tfsdk:"options"`
}

type alertSlackChannelModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type alertSlackDestModel struct {
	Title         types.String             `tfsdk:"title"`
	Message       types.String             `tfsdk:"message"`
	SlackChannels []alertSlackChannelModel `tfsdk:"slack_channels"`
}

type alertWebhookDestModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
}

type fieldPathModel struct {
	FieldName types.String `tfsdk:"field_name"`
	ModelID   types.String `tfsdk:"model_id"`
	DataSetID types.Int64  `tfsdk:"data_set_id"`
	IsMetric  types.Bool   `tfsdk:"is_metric"`
}

type vizConditionModel struct {
	FieldPath      *fieldPathModel `tfsdk:"field_path"`
	Aggregation    types.String    `tfsdk:"aggregation"`
	Transformation types.String    `tfsdk:"transformation"`
	Condition      *conditionModel `tfsdk:"condition"`
}

type dataAlertResourceModel struct {
	ID                   types.Int64                `tfsdk:"id"`
	Title                types.String               `tfsdk:"title"`
	SourceID             types.String               `tfsdk:"source_id"`
	SourceType           types.String               `tfsdk:"source_type"`
	Schedule             *scheduleModel             `tfsdk:"schedule"`
	DynamicFilterPresets []dynamicFilterPresetModel `tfsdk:"dynamic_filter_presets"`
	VizConditions        []vizConditionModel        `tfsdk:"viz_conditions"`
	EmailDest            *alertEmailDestModel       `tfsdk:"email_dest"`
	SlackDest            *alertSlackDestModel       `tfsdk:"slack_dest"`
	WebhookDest          *alertWebhookDestModel     `tfsdk:"webhook_dest"`
}

func NewDataAlertResource() resource.Resource {
	return &dataAlertResource{}
}

func (r *dataAlertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_alert"
}

func (r *dataAlertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Holistics data alert that fires when a metric meets a condition.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planUseStateForUnknown()}},
			"title": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanUseStateForUnknown()},
			},
			"source_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the alert source (DashboardWidget or VizBlock).",
			},
			"source_type": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf("DashboardWidget", "VizBlock")},
				Description: "Source type.",
			},
			"schedule": schema.SingleNestedAttribute{
				Required:   true,
				Attributes: scheduleAttribute(),
			},
			"dynamic_filter_presets": schema.ListNestedAttribute{
				Required:     true,
				NestedObject: dynamicFilterPresetNested(),
				Description:  "Dynamic filter presets (use an empty list `[]` if none).",
			},
			"viz_conditions": schema.ListNestedAttribute{
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
						"aggregation":    schema.StringAttribute{Optional: true},
						"transformation": schema.StringAttribute{Optional: true},
						"condition": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"operator": schema.StringAttribute{Required: true},
								"modifier": schema.StringAttribute{Optional: true},
								"values": schema.ListAttribute{
									ElementType:   types.StringType,
									Optional:      true,
									Computed:      true,
									PlanModifiers: []planmodifier.List{listplanUseStateForUnknown()},
								},
							},
						},
					},
				},
			},
			"email_dest": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"title":      schema.StringAttribute{Optional: true},
					"recipients": schema.ListAttribute{ElementType: types.StringType, Required: true},
					"options": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"body_text": schema.StringAttribute{Optional: true},
						},
					},
				},
			},
			"slack_dest": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"title":   schema.StringAttribute{Optional: true},
					"message": schema.StringAttribute{Optional: true},
					"slack_channels": schema.ListNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id":   schema.StringAttribute{Optional: true},
								"name": schema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			"webhook_dest": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"endpoint": schema.StringAttribute{Required: true},
				},
			},
		},
	}
}

func (r *dataAlertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dataAlertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dataAlertResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	input, diags := buildDataAlertInput(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	da, err := r.client.CreateDataAlert(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create data alert", err.Error())
		return
	}
	resp.Diagnostics.Append(writeDataAlertState(ctx, da, &resp.State)...)
}

func (r *dataAlertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dataAlertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	da, err := r.client.GetDataAlert(ctx, int(state.ID.ValueInt64()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read data alert", err.Error())
		return
	}
	resp.Diagnostics.Append(writeDataAlertState(ctx, da, &resp.State)...)
}

func (r *dataAlertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dataAlertResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	input, diags := buildDataAlertInput(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	da, err := r.client.UpdateDataAlert(ctx, int(state.ID.ValueInt64()), input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update data alert", err.Error())
		return
	}
	resp.Diagnostics.Append(writeDataAlertState(ctx, da, &resp.State)...)
}

func (r *dataAlertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dataAlertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDataAlert(ctx, int(state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete data alert", err.Error())
	}
}

func (r *dataAlertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected integer alert ID, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func buildDataAlertInput(ctx context.Context, m *dataAlertResourceModel) (client.DataAlert, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := client.DataAlert{
		Title:      stringPtrFromTF(m.Title),
		SourceID:   client.FlexibleID(m.SourceID.ValueString()),
		SourceType: m.SourceType.ValueString(),
	}
	if m.Schedule != nil {
		out.Schedule = client.Schedule{Repeat: m.Schedule.Repeat.ValueString(), Paused: m.Schedule.Paused.ValueBool()}
	}

	presets, d := dynamicFilterPresetsToAPI(ctx, m.DynamicFilterPresets)
	diags.Append(d...)
	if presets == nil {
		presets = []client.DynamicFilterPreset{}
	}
	out.DynamicFilterPresets = presets

	vc, d := vizConditionsToAPI(ctx, m.VizConditions)
	diags.Append(d...)
	out.VizConditions = vc

	dest, d := buildDataAlertDest(ctx, m)
	diags.Append(d...)
	out.Dest = dest

	return out, diags
}

func buildDataAlertDest(ctx context.Context, m *dataAlertResourceModel) (client.DataAlertDest, diag.Diagnostics) {
	var diags diag.Diagnostics
	set := 0
	if m.EmailDest != nil {
		set++
	}
	if m.SlackDest != nil {
		set++
	}
	if m.WebhookDest != nil {
		set++
	}
	if set != 1 {
		diags.AddError("Invalid destination", fmt.Sprintf("exactly one of email_dest, slack_dest, or webhook_dest must be set (got %d)", set))
		return client.DataAlertDest{}, diags
	}

	switch {
	case m.EmailDest != nil:
		recipients, d := listToStringSlice(ctx, m.EmailDest.Recipients)
		diags.Append(d...)
		fields := &client.DataAlertEmailDestFields{
			Title:      stringPtrFromTF(m.EmailDest.Title),
			Recipients: recipients,
		}
		if m.EmailDest.Options != nil {
			fields.Options = &client.DataAlertEmailOptions{BodyText: stringPtrFromTF(m.EmailDest.Options.BodyText)}
		}
		return client.DataAlertDest{Type: "EmailDest", Email: fields}, diags
	case m.SlackDest != nil:
		s := m.SlackDest
		chans := make([]client.DataAlertSlackChannel, len(s.SlackChannels))
		for i, c := range s.SlackChannels {
			chans[i] = client.DataAlertSlackChannel{ID: stringPtrFromTF(c.ID), Name: stringPtrFromTF(c.Name)}
		}
		return client.DataAlertDest{Type: "SlackDest", Slack: &client.DataAlertSlackDestFields{
			Title:         stringPtrFromTF(s.Title),
			Message:       stringPtrFromTF(s.Message),
			SlackChannels: chans,
		}}, diags
	case m.WebhookDest != nil:
		return client.DataAlertDest{Type: "WebhookDest", Webhook: &client.DataAlertWebhookDestFields{
			Endpoint: m.WebhookDest.Endpoint.ValueString(),
		}}, diags
	}
	return client.DataAlertDest{}, diags
}

func vizConditionsToAPI(ctx context.Context, in []vizConditionModel) ([]client.VizCondition, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in == nil {
		return nil, diags
	}
	out := make([]client.VizCondition, len(in))
	for i, vc := range in {
		c := client.VizCondition{
			Aggregation:    stringPtrFromTF(vc.Aggregation),
			Transformation: stringPtrFromTF(vc.Transformation),
		}
		if vc.FieldPath != nil {
			c.FieldPath = client.FieldPath{
				FieldName: vc.FieldPath.FieldName.ValueString(),
				ModelID:   client.FlexibleID(vc.FieldPath.ModelID.ValueString()),
			}
			if !vc.FieldPath.DataSetID.IsNull() && !vc.FieldPath.DataSetID.IsUnknown() {
				v := int(vc.FieldPath.DataSetID.ValueInt64())
				c.FieldPath.DataSetID = &v
			}
			c.FieldPath.IsMetric = boolPtrFromTF(vc.FieldPath.IsMetric)
		}
		if vc.Condition != nil {
			values, d := listToStringSlice(ctx, vc.Condition.Values)
			diags.Append(d...)
			cv := make([]client.ConditionValue, len(values))
			for j, v := range values {
				cv[j] = client.ConditionValueFromString(v)
			}
			c.Condition = client.Condition{
				Operator: vc.Condition.Operator.ValueString(),
				Modifier: stringPtrFromTF(vc.Condition.Modifier),
				Values:   cv,
			}
		}
		out[i] = c
	}
	return out, diags
}

func writeDataAlertState(ctx context.Context, da *client.DataAlert, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	if da.ID == nil {
		diags.AddError("Invalid data_alert response", "id missing")
		return diags
	}
	m := dataAlertResourceModel{
		ID:         types.Int64Value(int64(*da.ID)),
		Title:      stringFromPtr(da.Title),
		SourceID:   stringFromFlexibleID(da.SourceID),
		SourceType: types.StringValue(da.SourceType),
		Schedule: &scheduleModel{
			Repeat:     types.StringValue(da.Schedule.Repeat),
			Paused:     types.BoolValue(da.Schedule.Paused),
			IsArchived: boolFromPtr(da.Schedule.IsArchived),
		},
	}

	presets, d := dynamicFilterPresetsFromAPI(ctx, da.DynamicFilterPresets)
	diags.Append(d...)
	if presets == nil {
		presets = []dynamicFilterPresetModel{}
	}
	m.DynamicFilterPresets = presets

	vc, d := vizConditionsFromAPI(ctx, da.VizConditions)
	diags.Append(d...)
	m.VizConditions = vc

	d = readDataAlertDest(ctx, da.Dest, &m)
	diags.Append(d...)

	diags.Append(state.Set(ctx, &m)...)
	return diags
}

func vizConditionsFromAPI(ctx context.Context, in []client.VizCondition) ([]vizConditionModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in == nil {
		return nil, diags
	}
	out := make([]vizConditionModel, len(in))
	for i, vc := range in {
		vals := make([]string, 0, len(vc.Condition.Values))
		for _, v := range vc.Condition.Values {
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
			FieldName: types.StringValue(vc.FieldPath.FieldName),
			ModelID:   stringFromFlexibleID(vc.FieldPath.ModelID),
			IsMetric:  boolFromPtr(vc.FieldPath.IsMetric),
		}
		if vc.FieldPath.DataSetID != nil {
			fp.DataSetID = types.Int64Value(int64(*vc.FieldPath.DataSetID))
		} else {
			fp.DataSetID = types.Int64Null()
		}
		out[i] = vizConditionModel{
			FieldPath:      fp,
			Aggregation:    stringFromPtr(vc.Aggregation),
			Transformation: stringFromPtr(vc.Transformation),
			Condition: &conditionModel{
				Operator: types.StringValue(vc.Condition.Operator),
				Modifier: stringFromPtr(vc.Condition.Modifier),
				Values:   valsList,
			},
		}
	}
	return out, diags
}

func readDataAlertDest(ctx context.Context, d client.DataAlertDest, m *dataAlertResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	switch d.Type {
	case "EmailDest":
		if d.Email != nil {
			rec, dd := stringSliceToList(ctx, d.Email.Recipients)
			diags.Append(dd...)
			ed := &alertEmailDestModel{
				Title:      stringFromPtr(d.Email.Title),
				Recipients: rec,
			}
			if d.Email.Options != nil {
				ed.Options = &alertEmailOptionsModel{BodyText: stringFromPtr(d.Email.Options.BodyText)}
			}
			m.EmailDest = ed
		}
	case "SlackDest":
		if d.Slack != nil {
			chans := make([]alertSlackChannelModel, len(d.Slack.SlackChannels))
			for i, c := range d.Slack.SlackChannels {
				chans[i] = alertSlackChannelModel{ID: stringFromPtr(c.ID), Name: stringFromPtr(c.Name)}
			}
			m.SlackDest = &alertSlackDestModel{
				Title:         stringFromPtr(d.Slack.Title),
				Message:       stringFromPtr(d.Slack.Message),
				SlackChannels: chans,
			}
		}
	case "WebhookDest":
		if d.Webhook != nil {
			m.WebhookDest = &alertWebhookDestModel{Endpoint: types.StringValue(d.Webhook.Endpoint)}
		}
	}
	return diags
}
