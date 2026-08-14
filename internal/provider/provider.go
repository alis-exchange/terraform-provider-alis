package provider

import (
	"context"
	"crypto/sha256"
	"strings"
	"sync"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner"
	"terraform-provider-alis/internal/spanner/conn"
	spannerservices "terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	googleoauth "golang.org/x/oauth2/google"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider              = &googleProvider{}
	_ provider.ProviderWithFunctions = &googleProvider{}
)

// Every Configure call builds a provider instance, and each Connection owns a
// database admin client plus a session pool per database. The framework offers
// no teardown hook to close them, and Terraform reconfigures the provider
// freely — acceptance tests do it once per step — so connections are shared
// per distinct credential configuration and live for the process.
//
// This bounds the admin clients and gRPC channels to one per configuration; it
// does not bound what accumulates inside one. A Connection caches a session
// pool per database it is asked about and only releases them on Close, so a
// process touching many databases still grows — just far more slowly than one
// adapter per Configure did.
var (
	connectionsMu sync.Mutex
	connections   = map[[sha256.Size]byte]conn.Connection{}
)

// sharedConnection returns the Connection for this configuration, building it
// on first use.
//
// build runs under the lock, which is safe only because conn.New performs no
// I/O — it allocates the adapter and lets the clients come up lazily. Should
// that change, this serializes every provider configuration in the process.
func sharedConnection(key [sha256.Size]byte, build func() conn.Connection) conn.Connection {
	connectionsMu.Lock()
	defer connectionsMu.Unlock()

	if existing, ok := connections[key]; ok {
		return existing
	}

	cn := build()
	connections[key] = cn

	return cn
}

// connectionKey fingerprints everything the resulting Connection depends on:
// the configured credentials, the identity those resolved to — Application
// Default Credentials can name a different principal without the configuration
// changing at all — and the emulator host conn captures at construction. A key
// that misses any of them hands back a Connection built for another backend or
// another principal.
//
// The inputs are hashed rather than stored: this key lives in a map that
// survives for the process, which is no place for resolved credentials.
func connectionKey(project, credentials, accessToken string, resolved *googleoauth.Credentials) [sha256.Size]byte {
	// Configure rejects nil credentials before reaching here; treat them as
	// absent rather than panicking if that ever stops being true.
	var resolvedProject string
	var resolvedJSON []byte
	if resolved != nil {
		resolvedProject, resolvedJSON = resolved.ProjectID, resolved.JSON
	}

	return sha256.Sum256([]byte(strings.Join([]string{
		project,
		credentials,
		accessToken,
		conn.EmulatorHost(),
		resolvedProject,
		string(resolvedJSON),
	}, "\x00")))
}

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
				MarkdownDescription: "A JSON string of Google Cloud credentials.",
			},
			"access_token": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "An OAuth2 access token used to authenticate to Google Cloud instead of `credentials`.\n" +
					"Requires `project` to be set and conflicts with `credentials`.",
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
				Optional:            true,
				MarkdownDescription: "The Google Cloud project ID.",
			},
		},
		MarkdownDescription: "Custom terraform provider for managing various google resources used in ALIS.",
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
		resp.Diagnostics.AddError(
			"Unable to Resolve Google Cloud Credentials",
			"Ensure that either credentials or access_token is specified, or that the provider is running in an environment with Application Default Credentials: "+utils.ErrDetail(
				err,
			),
		)
		return
	}
	if googleCreds == nil {
		resp.Diagnostics.AddError(
			"Missing Google Cloud Credentials",
			"No credentials were resolved. Specify either credentials or access_token, or run the provider in an environment with Application Default Credentials.",
		)
		return
	}

	// Make the Bigtable and Spanner services available during DataSource and Resource
	// type Configure methods.
	providerConfig := &internal.ProviderConfig{
		GoogleProjectId: config.Project.ValueString(),
		// conn.New is the single place the resolved credentials reach every
		// Spanner client.
		SpannerService: spannerservices.NewSpannerService(
			sharedConnection(
				connectionKey(config.Project.ValueString(), credentials, accessToken, googleCreds),
				func() conn.Connection {
					return conn.New(conn.Options{Credentials: googleCreds})
				},
			),
		),
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

// Functions defines the provider-defined functions implemented in the
// provider. They require Terraform 1.8 or later.
func (p *googleProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{
		spanner.NewProtoTimestampDdlFunction,
		spanner.NewResourceNameAncestorDdlFunction,
		spanner.NewResourceNameIDDdlFunction,
	}
}
