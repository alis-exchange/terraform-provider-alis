package spanner

import (
	"context"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &databaseRoleResource{}
	_ resource.ResourceWithConfigure   = &databaseRoleResource{}
	_ resource.ResourceWithImportState = &databaseRoleResource{}
)

// NewDatabaseRoleResource is a helper function to simplify the provider implementation.
func NewDatabaseRoleResource() resource.Resource {
	return &databaseRoleResource{}
}

type databaseRoleResource struct {
	config *internal.ProviderConfig
}

type databaseRoleModel struct {
	Project  types.String   `tfsdk:"project"`
	Instance types.String   `tfsdk:"instance"`
	Database types.String   `tfsdk:"database"`
	Role     types.String   `tfsdk:"role"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// Metadata returns the resource type name.
func (r *databaseRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database_role"
}

// Schema defines the schema for the resource.
func (r *databaseRoleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "The role that should be applied.",
			},
		},
		Description: "Creates a custom role in the database if it does not exist. If the role already exists, it will be imported into the state.\n" +
			"Authoritative for a given role. Other roles within the database are preserved.",
	}
}

// Create ensures the role exists: a role already present in the database is
// adopted into state as-is rather than treated as a conflict; otherwise
// CREATE ROLE DDL is issued.
func (r *databaseRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan databaseRoleModel
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
	role := plan.Role.ValueString()

	databaseName := names.DatabaseName{Project: project, Instance: instance, Database: database}.String()
	roleName := names.DatabaseRoleName{Project: project, Instance: instance, Database: database, Role: role}.String()

	existingRole, err := r.config.SpannerService.GetDatabaseRole(ctx, roleName)
	if err != nil && status.Code(err) != codes.NotFound {
		resp.Diagnostics.AddError(
			"Error Checking Existing Database Role",
			"Could not verify whether Role ("+roleName+") already exists: "+utils.ErrDetail(err),
		)
		return
	}
	if existingRole != nil {
		// Set state to fully populated data
		diags = resp.State.Set(ctx, plan)
		resp.Diagnostics.Append(diags...)
		return
	}

	_, err = r.config.SpannerService.CreateDatabaseRole(ctx, databaseName, role)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Database Role",
			"Could not create Role ("+role+") in Database ("+databaseName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *databaseRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state databaseRoleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()
	role := state.Role.ValueString()

	roleName := names.DatabaseRoleName{Project: project, Instance: instance, Database: database, Role: role}.String()

	_, err := r.config.SpannerService.GetDatabaseRole(ctx, roleName)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Database Role",
			"Could not read Role ("+roleName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *databaseRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan databaseRoleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Database role is immutable, so we need to delete and recreate it
	// This is already handled by the plan engine, so we just need to return here

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *databaseRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state databaseRoleModel
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
	role := state.Role.ValueString()

	roleName := names.DatabaseRoleName{Project: project, Instance: instance, Database: database, Role: role}.String()

	err := r.config.SpannerService.DeleteDatabaseRole(ctx, roleName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Database Role",
			"Could not delete Role ("+roleName+"): "+utils.ErrDetail(err),
		)
		return
	}
}

func (r *databaseRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importName, err := names.ParseDatabaseRole(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") must be in the format projects/{project}/instances/{instance}/databases/{database}/databaseRoles/{role}: "+err.Error(),
		)
		return
	}
	project := importName.Project
	instanceName := importName.Instance
	databaseName := importName.Database
	role := importName.Role

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), role)...)
}

// Configure adds the provider configured client to the resource.
func (r *databaseRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

func (r *databaseRoleResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
