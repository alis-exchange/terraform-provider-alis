package spanner

import (
	"context"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &tableIamBindingDataSource{}
	_ datasource.DataSourceWithConfigure = &tableIamBindingDataSource{}
)

// NewTableIamBindingDataSource is a helper function to simplify the provider implementation.
func NewTableIamBindingDataSource() datasource.DataSource {
	return &tableIamBindingDataSource{}
}

type tableIamBindingDataSource struct {
	config *internal.ProviderConfig
}

// tableIamBindingModel backs the table IAM binding data source. The resource
// uses its own tableIamBindingResourceModel, which adds the timeouts block —
// a resource-only concept that must not appear in the data source schema.
type tableIamBindingModel struct {
	Project     types.String   `tfsdk:"project"`
	Instance    types.String   `tfsdk:"instance"`
	Database    types.String   `tfsdk:"database"`
	Table       types.String   `tfsdk:"table"`
	Role        types.String   `tfsdk:"role"`
	Permissions []types.String `tfsdk:"permissions"`
}

// Metadata returns the resource type name.
func (r *tableIamBindingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_iam_binding"
}

// Schema defines the schema for the resource.
func (r *tableIamBindingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
			"table": schema.StringAttribute{
				Required: true,
			},
			"role": schema.StringAttribute{
				Required:    true,
				Description: "The role that should be granted to the table.",
			},
			"permissions": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The permissions that should be granted to the role.\n" +
					"Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			},
		},
		Description: "Authoritative for a given role. Updates the table IAM policy to grant a role along with permissions.\n" +
			"Other roles and permissions within the IAM policy for the table are preserved.",
	}
}

// Read resource information.
func (r *tableIamBindingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state tableIamBindingModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()
	table := state.Table.ValueString()
	role := state.Role.ValueString()

	tableName := names.TableName{Project: project, Instance: instance, Database: database, Table: table}.String()

	binding, err := r.config.SpannerService.GetTableIamBinding(ctx, tableName, role)
	if err != nil {
		// A missing binding is an error for a data source: silently returning
		// null permissions would hide a misconfigured role reference.
		resp.Diagnostics.AddError(
			"Error Reading Table IAM Binding",
			"Could not read IAM binding for Role ("+role+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Map response body to state
	if binding != nil && binding.Permissions != nil {
		state.Permissions = make([]types.String, 0)

		for _, permission := range binding.Permissions {
			state.Permissions = append(state.Permissions, types.StringValue(permission.String()))
		}
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *tableIamBindingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

func (r *tableIamBindingDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
