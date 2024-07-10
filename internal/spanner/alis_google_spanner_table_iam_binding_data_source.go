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
			"permissions": schema.ListAttribute{
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

	binding, err := r.config.SpannerService.GetTableIamBinding(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instance, database, table),
		role,
	)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			resp.Diagnostics.AddError(
				"Error Reading IAM Policy",
				"Could not read IAM Policy for Table ("+state.Table.ValueString()+"): "+err.Error(),
			)
			return
		}
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
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*internal.ProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *utils.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

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
