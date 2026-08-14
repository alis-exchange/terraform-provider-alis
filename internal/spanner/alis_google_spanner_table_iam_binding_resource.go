package spanner

import (
	"context"
	"regexp"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &tableIamBindingResource{}
	_ resource.ResourceWithConfigure   = &tableIamBindingResource{}
	_ resource.ResourceWithImportState = &tableIamBindingResource{}
)

// NewTableIamBindingResource is a helper function to simplify the provider implementation.
func NewTableIamBindingResource() resource.Resource {
	return &tableIamBindingResource{}
}

type tableIamBindingResource struct {
	config *internal.ProviderConfig
}

// tableIamBindingResourceModel mirrors tableIamBindingModel (shared with the
// data source) plus the timeouts block, which only resources support.
type tableIamBindingResourceModel struct {
	Project     types.String   `tfsdk:"project"`
	Instance    types.String   `tfsdk:"instance"`
	Database    types.String   `tfsdk:"database"`
	Table       types.String   `tfsdk:"table"`
	Role        types.String   `tfsdk:"role"`
	Permissions []types.String `tfsdk:"permissions"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

// Metadata returns the resource type name.
func (r *tableIamBindingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_iam_binding"
}

// Schema defines the schema for the resource.
func (r *tableIamBindingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Google Cloud project ID containing the Spanner instance and database.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Spanner instance ID that contains the database.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Spanner database ID that contains the table.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"table": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The table the role and permissions are granted on.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						utils.Pattern(utils.SpannerGoogleSqlRoleIdRegex),
						utils.Pattern(utils.SpannerPostgresSqlRoleIdRegex),
					}, "Role must be a valid Spanner database role ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "The role that should be granted to the table.\n" +
					"The role must satisfy the expression `^[a-zA-Z0-9_]{1,64}$`.",
			},
			"permissions": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(services.SpannerTablePolicyBindingPermissions...)),
				},
				MarkdownDescription: "The permissions that should be granted to the role.\n" +
					"Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			},
		},
		MarkdownDescription: "Authoritative for a given role. Updates the table IAM policy to grant a role along with permissions.\n" +
			"Other roles and permissions within the IAM policy for the table are preserved.",
	}
}

// Create a new resource.
func (r *tableIamBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan tableIamBindingResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, createTimeout)
	defer cancel()

	// Retrieve project, instance and database from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	database := plan.Database.ValueString()
	table := plan.Table.ValueString()
	role := plan.Role.ValueString()

	permissions := make([]services.TablePolicyBindingPermission, 0)
	for _, permission := range plan.Permissions {
		switch permission.ValueString() {
		case "SELECT":
			permissions = append(permissions, services.TablePolicyBindingPermission_SELECT)
		case "INSERT":
			permissions = append(permissions, services.TablePolicyBindingPermission_INSERT)
		case "UPDATE":
			permissions = append(permissions, services.TablePolicyBindingPermission_UPDATE)
		case "DELETE":
			permissions = append(permissions, services.TablePolicyBindingPermission_DELETE)
		default:
			// Unreachable while the schema validator enforces the same set of
			// values; kept as a guard against the two drifting apart.
			resp.Diagnostics.AddAttributeError(
				path.Root("permissions"),
				"Invalid Permission",
				"Invalid permission ("+permission.ValueString()+") provided. Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			)
			return
		}
	}

	tableName := names.TableName{Project: project, Instance: instance, Database: database, Table: table}.String()

	binding, err := r.config.SpannerService.SetTableIamBinding(ctx,
		tableName,
		&services.TablePolicyBinding{
			Role:        role,
			Permissions: permissions,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Table IAM Binding",
			"Could not create IAM binding for Role ("+role+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Map response body to state
	if binding.Permissions != nil {
		plan.Permissions = make([]types.String, 0)

		for _, permission := range binding.Permissions {
			plan.Permissions = append(plan.Permissions, types.StringValue(permission.String()))
		}
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *tableIamBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state tableIamBindingResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()
	table := state.Table.ValueString()
	role := state.Role.ValueString()

	tableName := names.TableName{Project: project, Instance: instance, Database: database, Table: table}.String()

	binding, err := r.config.SpannerService.GetTableIamBinding(ctx, tableName, role)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Table IAM Binding",
			"Could not read IAM binding for Role ("+role+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Map response body to state
	if binding.Permissions != nil {
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

func (r *tableIamBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan tableIamBindingResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, updateTimeout)
	defer cancel()

	// Retrieve project, instance and database from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	database := plan.Database.ValueString()
	table := plan.Table.ValueString()
	role := plan.Role.ValueString()

	permissions := make([]services.TablePolicyBindingPermission, 0)
	for _, permission := range plan.Permissions {
		switch permission.ValueString() {
		case "SELECT":
			permissions = append(permissions, services.TablePolicyBindingPermission_SELECT)
		case "INSERT":
			permissions = append(permissions, services.TablePolicyBindingPermission_INSERT)
		case "UPDATE":
			permissions = append(permissions, services.TablePolicyBindingPermission_UPDATE)
		case "DELETE":
			permissions = append(permissions, services.TablePolicyBindingPermission_DELETE)
		default:
			// Unreachable while the schema validator enforces the same set of
			// values; kept as a guard against the two drifting apart.
			resp.Diagnostics.AddAttributeError(
				path.Root("permissions"),
				"Invalid Permission",
				"Invalid permission ("+permission.ValueString()+") provided. Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			)
			return
		}
	}

	tableName := names.TableName{Project: project, Instance: instance, Database: database, Table: table}.String()

	binding, err := r.config.SpannerService.SetTableIamBinding(ctx,
		tableName,
		&services.TablePolicyBinding{
			Role:        role,
			Permissions: permissions,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Table IAM Binding",
			"Could not update IAM binding for Role ("+role+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Map response body to state
	if binding.Permissions != nil {
		plan.Permissions = make([]types.String, 0)

		for _, permission := range binding.Permissions {
			plan.Permissions = append(plan.Permissions, types.StringValue(permission.String()))
		}
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *tableIamBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state tableIamBindingResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, deleteTimeout)
	defer cancel()

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()
	table := state.Table.ValueString()
	role := state.Role.ValueString()

	tableName := names.TableName{Project: project, Instance: instance, Database: database, Table: table}.String()

	err := r.config.SpannerService.DeleteTableIamBinding(ctx, tableName, role)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Table IAM Binding",
			"Could not delete IAM binding for Role ("+role+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}
}

func (r *tableIamBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importName, err := names.ParseTableRole(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}/tableRoles/{role}: "+err.Error(),
		)
		return
	}

	if !utils.Pattern(utils.SpannerGoogleSqlTableRoleNameRegex).MatchString(req.ID) &&
		!utils.Pattern(utils.SpannerPostgresSqlTableRoleNameRegex).MatchString(req.ID) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") contains an invalid project, instance, database, table or role ID. Expected format: projects/{project}/instances/{instance}/databases/{database}/tables/{table}/tableRoles/{role}.",
		)
		return
	}

	project := importName.Project
	instanceName := importName.Instance
	databaseName := importName.Database
	tableName := importName.Table
	role := importName.Role

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), tableName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), role)...)
}

// Configure adds the provider configured client to the resource.
func (r *tableIamBindingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

func (r *tableIamBindingResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
