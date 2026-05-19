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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var (
	_ resource.Resource                = (*dataScheduleResource)(nil)
	_ resource.ResourceWithConfigure   = (*dataScheduleResource)(nil)
	_ resource.ResourceWithImportState = (*dataScheduleResource)(nil)
)

type dataScheduleResource struct {
	client *client.Client
}

type scheduleModel struct {
	Repeat     types.String `tfsdk:"repeat"`
	Paused     types.Bool   `tfsdk:"paused"`
	IsArchived types.Bool   `tfsdk:"is_archived"`
}

type conditionModel struct {
	Operator types.String `tfsdk:"operator"`
	Modifier types.String `tfsdk:"modifier"`
	Values   types.List   `tfsdk:"values"`
}

type dynamicFilterPresetModel struct {
	DynamicFilterID types.String    `tfsdk:"dynamic_filter_id"`
	PresetCondition *conditionModel `tfsdk:"preset_condition"`
	PublicHidden    types.Bool      `tfsdk:"public_hidden"`
}

type emailOptionsModel struct {
	Preview           types.Bool   `tfsdk:"preview"`
	IncludeHeader     types.Bool   `tfsdk:"include_header"`
	IncludeReportLink types.Bool   `tfsdk:"include_report_link"`
	IncludeFilters    types.Bool   `tfsdk:"include_filters"`
	DontSendWhenEmpty types.Bool   `tfsdk:"dont_send_when_empty"`
	BodyText          types.String `tfsdk:"body_text"`
	SelectedType      types.String `tfsdk:"selected_type"`
	SelectedValues    types.List   `tfsdk:"selected_values"`
	AttachmentFormats types.List   `tfsdk:"attachment_formats"`
}

type emailDestModel struct {
	Title      types.String       `tfsdk:"title"`
	Recipients types.List         `tfsdk:"recipients"`
	Options    *emailOptionsModel `tfsdk:"options"`
}

type sftpDestModel struct {
	DashboardWidgetID types.String `tfsdk:"dashboard_widget_id"`
	DataSourceID      types.Int64  `tfsdk:"data_source_id"`
	FilePath          types.String `tfsdk:"file_path"`
	Format            types.String `tfsdk:"format"`
	IncludeHeader     types.Bool   `tfsdk:"include_header"`
	Separator         types.String `tfsdk:"separator"`
}

type slackChannelModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`
}

type slackDestModel struct {
	HideFilters   types.Bool          `tfsdk:"hide_filters"`
	Message       types.String        `tfsdk:"message"`
	Title         types.String        `tfsdk:"title"`
	SlackChannels []slackChannelModel `tfsdk:"slack_channels"`
}

type gsheetDestModel struct {
	SheetURL   types.String `tfsdk:"sheet_url"`
	SheetTitle types.String `tfsdk:"sheet_title"`
}

type dataScheduleResourceModel struct {
	ID                    types.Int64                `tfsdk:"id"`
	Title                 types.String               `tfsdk:"title"`
	SourceType            types.String               `tfsdk:"source_type"`
	SourceID              types.Int64                `tfsdk:"source_id"`
	Schedule              *scheduleModel             `tfsdk:"schedule"`
	DynamicFilterPresets  []dynamicFilterPresetModel `tfsdk:"dynamic_filter_presets"`
	EmailDest             *emailDestModel            `tfsdk:"email_dest"`
	EmailSubscriptionDest *emailDestModel            `tfsdk:"email_subscription_dest"`
	SftpDest              *sftpDestModel             `tfsdk:"sftp_dest"`
	SlackDest             *slackDestModel            `tfsdk:"slack_dest"`
	GoogleSheetDest       *gsheetDestModel           `tfsdk:"google_sheet_dest"`
}

func NewDataScheduleResource() resource.Resource {
	return &dataScheduleResource{}
}

func (r *dataScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_schedule"
}

func (r *dataScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Holistics data schedule (Dashboard or QueryReport delivered via email, Slack, SFTP, or Google Sheets).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:      true,
				Description:   "Data schedule ID.",
				PlanModifiers: []planmodifier.Int64{int64planUseStateForUnknown()},
			},
			"title": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Schedule title. Defaults to the source title.",
			},
			"source_type": schema.StringAttribute{
				Required:    true,
				Description: "Source object type. One of `Dashboard` or `QueryReport`.",
				Validators:  []validator.String{stringvalidator.OneOf("Dashboard", "QueryReport")},
			},
			"source_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the Dashboard or QueryReport.",
			},
			"schedule": schema.SingleNestedAttribute{
				Required:    true,
				Description: "When the schedule fires.",
				Attributes:  scheduleAttribute(),
			},
			"dynamic_filter_presets": schema.ListNestedAttribute{
				Optional:     true,
				Description:  "Dynamic filter presets applied before delivery.",
				NestedObject: dynamicFilterPresetNested(),
			},
			"email_dest":              emailDestAttribute("Email destination."),
			"email_subscription_dest": emailDestAttribute("Email subscription destination."),
			"sftp_dest":               sftpDestAttribute(),
			"slack_dest":              slackDestAttribute(),
			"google_sheet_dest":       gsheetDestAttribute(),
		},
	}
}

func scheduleAttribute() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"repeat": schema.StringAttribute{
			Required:    true,
			Description: "Crontab expression (e.g. `0 4 * * *`).",
		},
		"paused": schema.BoolAttribute{
			Required:    true,
			Description: "Pause execution.",
		},
		"is_archived": schema.BoolAttribute{
			Computed:    true,
			Description: "True if paused because the source was archived.",
		},
	}
}

func dynamicFilterPresetNested() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"dynamic_filter_id": schema.StringAttribute{
				Required:    true,
				Description: "Dynamic filter ID (numeric or string).",
			},
			"preset_condition": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"operator": schema.StringAttribute{Required: true},
					"modifier": schema.StringAttribute{Optional: true},
					"values": schema.ListAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Computed:    true,
					},
				},
			},
			"public_hidden": schema.BoolAttribute{Optional: true},
		},
	}
}

func emailOptionsAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"preview":              schema.BoolAttribute{Optional: true},
			"include_header":       schema.BoolAttribute{Optional: true},
			"include_report_link":  schema.BoolAttribute{Optional: true},
			"include_filters":      schema.BoolAttribute{Optional: true},
			"dont_send_when_empty": schema.BoolAttribute{Optional: true},
			"body_text":            schema.StringAttribute{Optional: true},
			"selected_type":        schema.StringAttribute{Optional: true},
			"selected_values":      schema.ListAttribute{ElementType: types.StringType, Optional: true},
			"attachment_formats":   schema.ListAttribute{ElementType: types.StringType, Optional: true},
		},
	}
}

func emailDestAttribute(desc string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"title":      schema.StringAttribute{Optional: true},
			"recipients": schema.ListAttribute{ElementType: types.StringType, Required: true},
			"options":    emailOptionsAttribute(),
		},
	}
}

func sftpDestAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "SFTP destination.",
		Attributes: map[string]schema.Attribute{
			"dashboard_widget_id": schema.StringAttribute{Optional: true},
			"data_source_id":      schema.Int64Attribute{Required: true},
			"file_path":           schema.StringAttribute{Required: true},
			"format":              schema.StringAttribute{Optional: true},
			"include_header":      schema.BoolAttribute{Optional: true},
			"separator":           schema.StringAttribute{Optional: true},
		},
	}
}

func slackDestAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Slack destination.",
		Attributes: map[string]schema.Attribute{
			"hide_filters": schema.BoolAttribute{Optional: true},
			"message":      schema.StringAttribute{Optional: true},
			"title":        schema.StringAttribute{Optional: true},
			"slack_channels": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Optional: true},
						"name": schema.StringAttribute{Optional: true},
						"type": schema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}
}

func gsheetDestAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Google Sheets destination.",
		Attributes: map[string]schema.Attribute{
			"sheet_url":   schema.StringAttribute{Required: true},
			"sheet_title": schema.StringAttribute{Required: true},
		},
	}
}

func (r *dataScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dataScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dataScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := buildDataScheduleInput(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.client.CreateDataSchedule(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create data schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(writeDataScheduleState(ctx, ds, &resp.State)...)
}

func (r *dataScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dataScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds, err := r.client.GetDataSchedule(ctx, int(state.ID.ValueInt64()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read data schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(writeDataScheduleState(ctx, ds, &resp.State)...)
}

func (r *dataScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dataScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := buildDataScheduleInput(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.client.UpdateDataSchedule(ctx, int(state.ID.ValueInt64()), input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update data schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(writeDataScheduleState(ctx, ds, &resp.State)...)
}

func (r *dataScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dataScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDataSchedule(ctx, int(state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete data schedule", err.Error())
	}
}

func (r *dataScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected integer schedule ID, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func buildDataScheduleInput(ctx context.Context, m *dataScheduleResourceModel) (client.DataSchedule, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := client.DataSchedule{
		Title:      stringPtrFromTF(m.Title),
		SourceType: m.SourceType.ValueString(),
		SourceID:   int(m.SourceID.ValueInt64()),
	}
	if m.Schedule != nil {
		out.Schedule = client.Schedule{
			Repeat: m.Schedule.Repeat.ValueString(),
			Paused: m.Schedule.Paused.ValueBool(),
		}
	}

	presets, d := dynamicFilterPresetsToAPI(ctx, m.DynamicFilterPresets)
	diags.Append(d...)
	out.DynamicFilterPresets = presets

	dest, d := buildDataScheduleDest(ctx, m)
	diags.Append(d...)
	out.Dest = dest

	return out, diags
}

func buildDataScheduleDest(ctx context.Context, m *dataScheduleResourceModel) (client.DataScheduleDest, diag.Diagnostics) {
	var diags diag.Diagnostics
	dests := 0
	if m.EmailDest != nil {
		dests++
	}
	if m.EmailSubscriptionDest != nil {
		dests++
	}
	if m.SftpDest != nil {
		dests++
	}
	if m.SlackDest != nil {
		dests++
	}
	if m.GoogleSheetDest != nil {
		dests++
	}
	if dests != 1 {
		diags.AddError("Invalid destination", fmt.Sprintf("exactly one of email_dest, email_subscription_dest, sftp_dest, slack_dest, or google_sheet_dest must be set (got %d)", dests))
		return client.DataScheduleDest{}, diags
	}

	switch {
	case m.EmailDest != nil:
		f, d := emailDestToAPI(ctx, m.EmailDest)
		diags.Append(d...)
		return client.DataScheduleDest{Type: "EmailDest", Email: &client.DataScheduleEmailDestFields{
			Title:      f.Title,
			Recipients: f.Recipients,
			Options:    f.Options,
		}}, diags
	case m.EmailSubscriptionDest != nil:
		f, d := emailDestToAPI(ctx, m.EmailSubscriptionDest)
		diags.Append(d...)
		return client.DataScheduleDest{Type: "EmailSubscriptionDest", EmailSubscription: &client.DataScheduleEmailSubscriptionDestFields{
			Recipients: f.Recipients,
			Options:    f.Options,
		}}, diags
	case m.SftpDest != nil:
		s := m.SftpDest
		return client.DataScheduleDest{Type: "SftpDest", Sftp: &client.DataScheduleSftpDestFields{
			DashboardWidgetID: stringPtrFromTF(s.DashboardWidgetID),
			DataSourceID:      int(s.DataSourceID.ValueInt64()),
			FilePath:          s.FilePath.ValueString(),
			Format:            stringPtrFromTF(s.Format),
			IncludeHeader:     boolPtrFromTF(s.IncludeHeader),
			Separator:         stringPtrFromTF(s.Separator),
		}}, diags
	case m.SlackDest != nil:
		s := m.SlackDest
		channels := make([]client.SlackChannel, len(s.SlackChannels))
		for i, ch := range s.SlackChannels {
			channels[i] = client.SlackChannel{
				ID:   stringPtrFromTF(ch.ID),
				Name: stringPtrFromTF(ch.Name),
				Type: stringPtrFromTF(ch.Type),
			}
		}
		return client.DataScheduleDest{Type: "SlackDest", Slack: &client.DataScheduleSlackDestFields{
			HideFilters:   boolPtrFromTF(s.HideFilters),
			Message:       stringPtrFromTF(s.Message),
			Title:         stringPtrFromTF(s.Title),
			SlackChannels: channels,
		}}, diags
	case m.GoogleSheetDest != nil:
		g := m.GoogleSheetDest
		return client.DataScheduleDest{Type: "GsheetDest", GoogleSheet: &client.DataScheduleGoogleSheetDestFields{
			SheetURL:   g.SheetURL.ValueString(),
			SheetTitle: g.SheetTitle.ValueString(),
		}}, diags
	}
	return client.DataScheduleDest{}, diags
}

type emailDestAPI struct {
	Title      *string
	Recipients []string
	Options    *client.DataScheduleEmailOptions
}

func emailDestToAPI(ctx context.Context, m *emailDestModel) (emailDestAPI, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := emailDestAPI{Title: stringPtrFromTF(m.Title)}
	recipients, d := listToStringSlice(ctx, m.Recipients)
	diags.Append(d...)
	out.Recipients = recipients
	if m.Options != nil {
		opts := &client.DataScheduleEmailOptions{
			Preview:           boolPtrFromTF(m.Options.Preview),
			IncludeHeader:     boolPtrFromTF(m.Options.IncludeHeader),
			IncludeReportLink: boolPtrFromTF(m.Options.IncludeReportLink),
			IncludeFilters:    boolPtrFromTF(m.Options.IncludeFilters),
			DontSendWhenEmpty: boolPtrFromTF(m.Options.DontSendWhenEmpty),
			BodyText:          stringPtrFromTF(m.Options.BodyText),
			SelectedType:      stringPtrFromTF(m.Options.SelectedType),
		}
		sv, d := listToStringSlice(ctx, m.Options.SelectedValues)
		diags.Append(d...)
		opts.SelectedValues = stringSliceToJSONNumbers(sv)
		af, d := listToStringSlice(ctx, m.Options.AttachmentFormats)
		diags.Append(d...)
		opts.AttachmentFormats = af
		out.Options = opts
	}
	return out, diags
}

func stringSliceToJSONNumbers(in []string) []json.Number {
	if in == nil {
		return nil
	}
	out := make([]json.Number, len(in))
	for i, v := range in {
		out[i] = json.Number(v)
	}
	return out
}

func dynamicFilterPresetsToAPI(ctx context.Context, in []dynamicFilterPresetModel) ([]client.DynamicFilterPreset, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in == nil {
		return nil, diags
	}
	out := make([]client.DynamicFilterPreset, len(in))
	for i, p := range in {
		preset := client.DynamicFilterPreset{
			DynamicFilterID: json.Number(p.DynamicFilterID.ValueString()),
			PublicHidden:    boolPtrFromTF(p.PublicHidden),
		}
		if p.PresetCondition != nil {
			values, d := listToStringSlice(ctx, p.PresetCondition.Values)
			diags.Append(d...)
			cv := make([]client.ConditionValue, len(values))
			for j, v := range values {
				cv[j] = client.ConditionValueFromString(v)
			}
			preset.PresetCondition = client.Condition{
				Operator: p.PresetCondition.Operator.ValueString(),
				Modifier: stringPtrFromTF(p.PresetCondition.Modifier),
				Values:   cv,
			}
		}
		out[i] = preset
	}
	return out, diags
}

func dynamicFilterPresetsFromAPI(ctx context.Context, in []client.DynamicFilterPreset) ([]dynamicFilterPresetModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in == nil {
		return nil, diags
	}
	out := make([]dynamicFilterPresetModel, len(in))
	for i, p := range in {
		vals := make([]string, 0, len(p.PresetCondition.Values))
		for _, v := range p.PresetCondition.Values {
			switch {
			case v.String != nil:
				vals = append(vals, *v.String)
			case v.Number != nil:
				vals = append(vals, strconv.FormatFloat(*v.Number, 'f', -1, 64))
			case v.Bool != nil:
				vals = append(vals, strconv.FormatBool(*v.Bool))
			}
		}
		valuesList, d := stringSliceToList(ctx, vals)
		diags.Append(d...)
		out[i] = dynamicFilterPresetModel{
			DynamicFilterID: types.StringValue(string(p.DynamicFilterID)),
			PresetCondition: &conditionModel{
				Operator: types.StringValue(p.PresetCondition.Operator),
				Modifier: stringFromPtr(p.PresetCondition.Modifier),
				Values:   valuesList,
			},
			PublicHidden: boolFromPtr(p.PublicHidden),
		}
	}
	return out, diags
}

func writeDataScheduleState(ctx context.Context, ds *client.DataSchedule, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	if ds.ID == nil {
		diags.AddError("Invalid data_schedule response", "id missing")
		return diags
	}
	m := dataScheduleResourceModel{
		ID:         types.Int64Value(int64(*ds.ID)),
		Title:      stringFromPtr(ds.Title),
		SourceType: types.StringValue(ds.SourceType),
		SourceID:   types.Int64Value(int64(ds.SourceID)),
		Schedule: &scheduleModel{
			Repeat:     types.StringValue(ds.Schedule.Repeat),
			Paused:     types.BoolValue(ds.Schedule.Paused),
			IsArchived: boolFromPtr(ds.Schedule.IsArchived),
		},
	}

	presets, d := dynamicFilterPresetsFromAPI(ctx, ds.DynamicFilterPresets)
	diags.Append(d...)
	m.DynamicFilterPresets = presets

	d = readDataScheduleDest(ctx, ds.Dest, &m)
	diags.Append(d...)

	diags.Append(state.Set(ctx, &m)...)
	return diags
}

func readDataScheduleDest(ctx context.Context, d client.DataScheduleDest, m *dataScheduleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	switch d.Type {
	case "EmailDest":
		if d.Email != nil {
			ed, dd := emailDestFromAPI(ctx, d.Email.Title, d.Email.Recipients, d.Email.Options)
			diags.Append(dd...)
			m.EmailDest = &ed
		}
	case "EmailSubscriptionDest":
		if d.EmailSubscription != nil {
			ed, dd := emailDestFromAPI(ctx, nil, d.EmailSubscription.Recipients, d.EmailSubscription.Options)
			diags.Append(dd...)
			m.EmailSubscriptionDest = &ed
		}
	case "SftpDest":
		if d.Sftp != nil {
			m.SftpDest = &sftpDestModel{
				DashboardWidgetID: stringFromPtr(d.Sftp.DashboardWidgetID),
				DataSourceID:      types.Int64Value(int64(d.Sftp.DataSourceID)),
				FilePath:          types.StringValue(d.Sftp.FilePath),
				Format:            stringFromPtr(d.Sftp.Format),
				IncludeHeader:     boolFromPtr(d.Sftp.IncludeHeader),
				Separator:         stringFromPtr(d.Sftp.Separator),
			}
		}
	case "SlackDest":
		if d.Slack != nil {
			chans := make([]slackChannelModel, len(d.Slack.SlackChannels))
			for i, c := range d.Slack.SlackChannels {
				chans[i] = slackChannelModel{
					ID:   stringFromPtr(c.ID),
					Name: stringFromPtr(c.Name),
					Type: stringFromPtr(c.Type),
				}
			}
			m.SlackDest = &slackDestModel{
				HideFilters:   boolFromPtr(d.Slack.HideFilters),
				Message:       stringFromPtr(d.Slack.Message),
				Title:         stringFromPtr(d.Slack.Title),
				SlackChannels: chans,
			}
		}
	case "GsheetDest":
		if d.GoogleSheet != nil {
			m.GoogleSheetDest = &gsheetDestModel{
				SheetURL:   types.StringValue(d.GoogleSheet.SheetURL),
				SheetTitle: types.StringValue(d.GoogleSheet.SheetTitle),
			}
		}
	}
	return diags
}

func emailDestFromAPI(ctx context.Context, title *string, recipients []string, options *client.DataScheduleEmailOptions) (emailDestModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	recList, d := stringSliceToList(ctx, recipients)
	diags.Append(d...)
	out := emailDestModel{
		Title:      stringFromPtr(title),
		Recipients: recList,
	}
	if options != nil {
		var sv []string
		if options.SelectedValues != nil {
			sv = make([]string, len(options.SelectedValues))
			for i, n := range options.SelectedValues {
				sv[i] = string(n)
			}
		}
		svList, d := stringSliceToList(ctx, sv)
		diags.Append(d...)
		afList, d := stringSliceToList(ctx, options.AttachmentFormats)
		diags.Append(d...)
		out.Options = &emailOptionsModel{
			Preview:           boolFromPtr(options.Preview),
			IncludeHeader:     boolFromPtr(options.IncludeHeader),
			IncludeReportLink: boolFromPtr(options.IncludeReportLink),
			IncludeFilters:    boolFromPtr(options.IncludeFilters),
			DontSendWhenEmpty: boolFromPtr(options.DontSendWhenEmpty),
			BodyText:          stringFromPtr(options.BodyText),
			SelectedType:      stringFromPtr(options.SelectedType),
			SelectedValues:    svList,
			AttachmentFormats: afList,
		}
	}
	return out, diags
}
