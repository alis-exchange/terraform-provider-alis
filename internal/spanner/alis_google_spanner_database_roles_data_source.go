package spanner

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &databaseRolesDataSource{}
	_ datasource.DataSourceWithConfigure = &databaseRolesDataSource{}
)

// NewDatabaseRolesDataSource is a helper function to simplify the data source implementation.
func NewDatabaseRolesDataSource() datasource.DataSource {
	return &databaseRolesDataSource{}
}

type databaseRolesDataSource struct {
	config *internal.ProviderConfig
}

type databaseRolesModel struct {
	Project  types.String   `tfsdk:"project"`
	Instance types.String   `tfsdk:"instance"`
	Database types.String   `tfsdk:"database"`
	Roles    []types.String `tfsdk:"roles"`
}

// Metadata returns the resource type name.
func (d *databaseRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database_roles"
}

// Schema defines the schema for the resource.
func (d *databaseRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required: true,
			},
			"instance": schema.StringAttribute{
				Required: true,
			},
			"database": schema.StringAttribute{
				Required: true,
			},
			"roles": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Read resource information.
func (d *databaseRolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state databaseRolesModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()

	nextPageToken := ""
	roles := make([]string, 0)
	for {
		rolesRes, pageToken, err := d.config.SpannerService.ListDatabaseRoles(ctx,
			fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
			100, nextPageToken,
		)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				resp.State.RemoveResource(ctx)

				return
			}

			resp.Diagnostics.AddError("Failed to get Spanner Database IAM Policy", err.Error())
			return
		}

		for _, role := range rolesRes {
			roles = append(roles, role.Name)
		}
		nextPageToken = pageToken
		if nextPageToken == "" {
			break
		}
	}

	// Set refreshed state
	for _, role := range roles {
		state.Roles = append(state.Roles, types.StringValue(role))
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the resource.
func (d *databaseRolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*internal.ProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *utils.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.config = config
}

func (d *databaseRolesDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		//datasourcevalidator.All(),
		//	datasourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
