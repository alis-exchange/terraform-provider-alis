package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"go.alis.build/client"
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
	"terraform-provider-alis/internal/bigtable"
	"terraform-provider-alis/internal/spanner"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &bigtableProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &bigtableProvider{
			version: version,
		}
	}
}

// bigtableProvider is the provider implementation.
type bigtableProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// bigtableProviderModel maps provider schema data to a Go type.
type bigtableProviderModel struct {
	Host types.String `tfsdk:"host"`
}

// ProviderClients is a container for all provider clients.
type ProviderClients struct {
	Bigtable pb.BigtableServiceClient
	Spanner  pb.SpannerServiceClient
}

// Metadata returns the provider type name.
func (p *bigtableProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "alis"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *bigtableProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *bigtableProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring DB client")

	// Retrieve provider data from configuration
	var config bigtableProviderModel
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

	var bigtableServiceClient pb.BigtableServiceClient
	var spannerServiceClient pb.SpannerServiceClient

	// GOOGLE_APPLICATION_CREDENTIALS="/Users/newtonnthiga/Projects/Terraform/terraform-provider-alis-build/key-mentenova-db-prod-woi.json" terraform plan
	if conn, err := client.NewConn(ctx, host, true); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create DB API Client",
			"An unexpected error occurred when creating the DB API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"HashiCups Client Error: "+err.Error(),
		)
		return
	} else {
		bigtableServiceClient = pb.NewBigtableServiceClient(conn)
		spannerServiceClient = pb.NewSpannerServiceClient(conn)
	}

	// Make the DB client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = ProviderClients{
		Bigtable: bigtableServiceClient,
		Spanner:  spannerServiceClient,
	}
	resp.ResourceData = ProviderClients{
		Bigtable: bigtableServiceClient,
		Spanner:  spannerServiceClient,
	}

	tflog.Info(ctx, "Configured DB client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *bigtableProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

// Resources defines the resources implemented in the provider.
func (p *bigtableProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		bigtable.NewTableResource,
		bigtable.NewGarbageCollectionPolicyResource,
		spanner.NewSpannerDatabaseResource,
	}
}
