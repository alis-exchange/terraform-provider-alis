package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/bigtable"
	bigtableservices "terraform-provider-alis/internal/bigtable/services"
	"terraform-provider-alis/internal/spanner"
	spannerservices "terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &googleProvider{}
)

// NewProvider is a helper function to simplify provider server and testing implementation.
func NewProvider(version string) func() provider.Provider {
	return func() provider.Provider {
		return &googleProvider{
			version: version,
		}
	}
}

// googleProvider is the provider implementation.
type googleProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// googleProviderModel maps provider schema data to a Go type.
type googleProviderModel struct {
	Credentials types.String `tfsdk:"credentials"`
	AccessToken types.String `tfsdk:"access_token"`
	Project     types.String `tfsdk:"project"`
}

// Metadata returns the provider type name.
func (p *googleProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "alis"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *googleProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"credentials": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.Expressions{
							path.MatchRoot("access_token"),
						}...,
					),
					validators.GoogleCredentialsValidator(),
					validators.StringNotEmpty(),
				},
				Description: "A JSON string of Google Cloud credentials.",
			},
			"access_token": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.Expressions{
							path.MatchRoot("project"),
						}...,
					),
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("credentials"),
					}...),
					validators.StringNotEmpty(),
				},
			},
			"project": schema.StringAttribute{
				Optional:    true,
				Description: "The Google Cloud project ID.",
			},
		},
		Description: "Custom terraform provider for managing various google resources used in ALIS.",
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *googleProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring DB client")

	// Retrieve provider data from configuration
	var config googleProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Initializing alis provider")

	// Perform validation of provider configuration
	//if config.Host.IsUnknown() {
	//	resp.Diagnostics.AddAttributeError(
	//		path.Root("host"),
	//		"Unknown DB API Host",
	//		"The provider cannot create the DB API client as there is an unknown configuration value for the DB API host. "+
	//			"Either target apply the source of the value first, set the value statically in the configuration, or use the ALIS_OS_DB_HOST environment variable.",
	//	)
	//}
	//
	//if resp.Diagnostics.HasError() {
	//	return
	//}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	credentials := config.Credentials.ValueString()
	accessToken := config.AccessToken.ValueString()

	// Get Google Cloud credentials
	googleCreds, err := utils.GetGoogleCredentials(ctx, config.Project.ValueString(), credentials, accessToken)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Google Cloud credentials",
			"Ensure that either credentials or access_token is specified or that the plugin is running in an ADC environment: "+err.Error())
		return
	}
	if googleCreds == nil {
		resp.Diagnostics.AddError("Failed to get Google Cloud credentials",
			"Ensure that either credentials or access_token is specified or that the plugin is running in an ADC environment.")
		return
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Make the Bigtable and Spanner services available during DataSource and Resource
	// type Configure methods.
	providerConfig := &internal.ProviderConfig{
		GoogleProjectId: config.Project.ValueString(),
		BigtableService: bigtableservices.NewBigtableService(googleCreds),
		SpannerService:  spannerservices.NewSpannerService(googleCreds),
	}
	resp.DataSourceData = providerConfig
	resp.ResourceData = providerConfig

	tflog.Info(ctx, "Done initializing alis provider", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *googleProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		bigtable.NewIamPolicyDataSource,
		spanner.NewIamPolicyDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *googleProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		bigtable.NewTableResource,
		bigtable.NewGarbageCollectionPolicyResource,
		bigtable.NewIamPolicyResource,
		bigtable.NewIamBindingResource,
		bigtable.NewIamMemberResource,
		spanner.NewSpannerDatabaseResource,
		spanner.NewIamPolicyResource,
		spanner.NewIamBindingResource,
		spanner.NewIamMemberResource,
		spanner.NewSpannerTableResource,
		spanner.NewSpannerTableIndexResource,
	}
}
