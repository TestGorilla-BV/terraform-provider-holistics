package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/client"
)

var _ provider.Provider = (*HolisticsProvider)(nil)

type HolisticsProvider struct {
	version string
}

type holisticsProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	Region  types.String `tfsdk:"region"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &HolisticsProvider{version: version}
	}
}

func (p *HolisticsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "holistics"
	resp.Version = p.version
}

func (p *HolisticsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Holistics (https://holistics.io) resources via the v2 API.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "Holistics API key (sent as X-Holistics-Key). May be set via the HOLISTICS_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"region": schema.StringAttribute{
				Description: "Holistics region. One of `apac`, `us`, or `eu`. Defaults to `apac`. Ignored if `base_url` is set. May be set via HOLISTICS_REGION.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("apac", "us", "eu"),
				},
			},
			"base_url": schema.StringAttribute{
				Description: "Override the API base URL (e.g. for testing). May be set via HOLISTICS_BASE_URL.",
				Optional:    true,
			},
		},
	}
}

func (p *HolisticsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data holisticsProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Unknown api_key", "api_key must be known at apply time.")
	}
	if data.Region.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("region"), "Unknown region", "region must be known at apply time.")
	}
	if data.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("base_url"), "Unknown base_url", "base_url must be known at apply time.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := data.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("HOLISTICS_API_KEY")
	}
	region := data.Region.ValueString()
	if region == "" {
		region = os.Getenv("HOLISTICS_REGION")
	}
	baseURL := data.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("HOLISTICS_BASE_URL")
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing api_key",
			"Set the provider api_key attribute or the HOLISTICS_API_KEY environment variable.",
		)
		return
	}

	c, err := client.New(client.Options{
		APIKey:    apiKey,
		Region:    region,
		BaseURL:   baseURL,
		UserAgent: "terraform-provider-holistics/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to construct Holistics client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *HolisticsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGroupResource,
		NewUserResource,
		NewUserAttributeResource,
		NewDataScheduleResource,
		NewDataAlertResource,
		NewShareableLinkResource,
	}
}

func (p *HolisticsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewUserDataSource,
		NewUsersDataSource,
		NewCurrentUserDataSource,
		NewDashboardDataSource,
		NewDataSourceDataSource,
		NewTagsDataSource,
	}
}
