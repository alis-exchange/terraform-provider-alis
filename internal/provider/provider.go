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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"terraform-provider-alis/internal/bigtable"
	"terraform-provider-alis/internal/spanner"
	"terraform-provider-alis/internal/validators"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &googleProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
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
	Host        types.String `tfsdk:"host"`
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
			"host": schema.StringAttribute{
				Optional: true,
			},
			"credentials": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("access_token"),
					}...),
					validators.GoogleCredentialsValidator(),
					validators.StringNotEmpty(),
				},
			},
			"access_token": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("credentials"),
					}...),
					validators.StringNotEmpty(),
				},
			},
			"project": schema.StringAttribute{
				Optional: true,
			},
		},
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

	// Perform validation of provider configuration
	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown DB API Host",
			"The provider cannot create the DB API client as there is an unknown configuration value for the DB API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ALIS_OS_DB_HOST environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("ALIS_OS_DB_HOST")

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing DB API Host",
			"The provider cannot create the DB API client as there is a missing or empty value for the DB API host. "+
				"Set the host value in the configuration or use the ALIS_OS_DB_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "db_host", host)

	tflog.Debug(ctx, "Creating DB client")

	tflog.Info(ctx, "Configured DB client", map[string]any{"success": true})
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
	}
}
