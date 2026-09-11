package spanner

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &databaseRoleDataSource{}
	_ datasource.DataSourceWithConfigure = &databaseRoleDataSource{}
)

// NewDatabaseRoleDataSource is a helper function to simplify the provider implementation.
func NewDatabaseRoleDataSource() datasource.DataSource {
	return &databaseRoleDataSource{}
}

type databaseRoleDataSource struct {
	service *services.SpannerService
}

// databaseRoleDataModel has no computed attributes: a role carries nothing
// beyond its name, so the data source is an existence check.
type databaseRoleDataModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Database types.String `tfsdk:"database"`
	Role     types.String `tfsdk:"role"`
}

// Metadata returns the data source type name.
func (d *databaseRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database_role"
}

// Schema defines the schema for the data source.
func (d *databaseRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Google Cloud project ID containing the Spanner instance and database.",
			},
			"instance": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Spanner instance ID that contains the database.",
			},
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Spanner database ID that defines the role.",
			},
			"role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database role name to look up.",
			},
		},
		MarkdownDescription: "Verifies that a database role exists in a Cloud Spanner database. A role has no attributes beyond its name, " +
			"so this data source exposes nothing computed; reading a role that does not exist is an error, which makes it useful " +
			"as a guard (`depends_on`) for IAM bindings that reference roles managed elsewhere.",
	}
}

// Read verifies the role exists; any failure, including NotFound, is an error.
func (d *databaseRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseRoleDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	roleName := names.DatabaseRoleName{
		Project:  state.Project.ValueString(),
		Instance: state.Instance.ValueString(),
		Database: state.Database.ValueString(),
		Role:     state.Role.ValueString(),
	}.String()

	if _, err := d.service.GetDatabaseRole(ctx, roleName); err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Database Role",
			"Could not read Role ("+roleName+"): "+utils.ErrDetail(err),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *databaseRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
