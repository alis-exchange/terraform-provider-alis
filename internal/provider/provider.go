package provider

import (
	"context"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner"
	"terraform-provider-alis/internal/spanner/conn"
	spannerservices "terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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

// Configure resolves Google credentials and builds the shared ProviderConfig
// that every resource and data source receives via ProviderData. Credentials
// are resolved exactly once here — from the credentials attribute, the
// access_token attribute, or Application Default Credentials, in that order
// (utils.GetGoogleCredentials) — and reach every Spanner client through
// conn.New, the provider's single credential path.
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

	// Unknown values would silently read as empty strings below (e.g. an
	// unknown credentials attribute degrading to Application Default
	// Credentials), so reject them up front per the framework guidance.
	if config.Credentials.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("credentials"),
			"Unknown Google Credentials",
			"The provider cannot configure Google Cloud clients because credentials is only known after apply. "+
				"Target-apply the source of the value first, or set it statically in the configuration.",
		)
	}
	if config.AccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Unknown Google Access Token",
			"The provider cannot configure Google Cloud clients because access_token is only known after apply. "+
				"Target-apply the source of the value first, or set it statically in the configuration.",
		)
	}
	if config.Project.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("project"),
			"Unknown Google Cloud Project",
			"The provider cannot configure Google Cloud clients because project is only known after apply. "+
				"Target-apply the source of the value first, or set it statically in the configuration.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Read configured values
	credentials := config.Credentials.ValueString()
	accessToken := config.AccessToken.ValueString()

	// Get Google Cloud credentials
	googleCreds, err := utils.GetGoogleCredentials(ctx, config.Project.ValueString(), credentials, accessToken)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Google Cloud Credentials",
			"Ensure that either credentials or access_token is specified, or that the provider is running in an environment with Application Default Credentials: "+utils.ErrDetail(err))
		return
	}
	if googleCreds == nil {
		resp.Diagnostics.AddError("Missing Google Cloud Credentials",
			"No credentials were resolved. Specify either credentials or access_token, or run the provider in an environment with Application Default Credentials.")
		return
	}

	// Make the Bigtable and Spanner services available during DataSource and Resource
	// type Configure methods.
	providerConfig := &internal.ProviderConfig{
		GoogleProjectId: config.Project.ValueString(),
		// conn.New is the single place the resolved credentials reach every
		// Spanner client.
		SpannerService: spannerservices.NewSpannerService(conn.New(conn.Options{Credentials: googleCreds})),
	}
	resp.DataSourceData = providerConfig
	resp.ResourceData = providerConfig

	tflog.Info(ctx, "Done initializing alis provider", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *googleProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		spanner.NewDatabaseRolesDataSource,
		spanner.NewTableIamBindingDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *googleProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		spanner.NewSpannerTableResource,
		spanner.NewSpannerTableIndexResource,
		spanner.NewTableForeignKeyResource,
		spanner.NewDatabaseRoleResource,
		spanner.NewTableIamBindingResource,
		spanner.NewTableTtlPolicyResource,
		spanner.NewDatabaseSequenceResource,
	}
}
