package spanner

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/services"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &tableIamBindingResource{}
	_ resource.ResourceWithConfigure = &tableIamBindingResource{}
)

// NewTableIamBindingResource is a helper function to simplify the provider implementation.
func NewTableIamBindingResource() resource.Resource {
	return &tableIamBindingResource{}
}

type tableIamBindingResource struct {
	config *internal.ProviderConfig
}

// Metadata returns the resource type name.
func (r *tableIamBindingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_iam_binding"
}

// Schema defines the schema for the resource.
func (r *tableIamBindingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
			"table": schema.StringAttribute{
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
				Description: "The role that should be granted to the table.",
			},
			"permissions": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf(services.SpannerTablePolicyBindingPermissions...)),
				},
				Description: "The permissions that should be granted to the role.\n" +
					"Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			},
		},
		Description: "Authoritative for a given role. Updates the table IAM policy to grant a role along with permissions.\n" +
			"Other roles and permissions within the IAM policy for the table are preserved.",
	}
}

// Create a new resource.
func (r *tableIamBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan tableIamBindingModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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
			resp.Diagnostics.AddError(
				"Invalid Permission",
				"Invalid permission ("+permission.ValueString()+") provided. Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			)
			return
		}
	}

	binding, err := r.config.SpannerService.SetTableIamBinding(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instance, database, table),
		&services.TablePolicyBinding{
			Role:        role,
			Permissions: permissions,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy Binding for ("+role+") in Table ("+plan.Table.ValueString()+"): "+err.Error(),
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
	var state tableIamBindingModel
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

	binding, err := r.config.SpannerService.GetTableIamBinding(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instance, database, table),
		role,
	)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Table ("+state.Table.ValueString()+"): "+err.Error(),
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
	var plan tableIamBindingModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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
			resp.Diagnostics.AddError(
				"Invalid Permission",
				"Invalid permission ("+permission.ValueString()+") provided. Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.",
			)
			return
		}
	}

	binding, err := r.config.SpannerService.SetTableIamBinding(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instance, database, table),
		&services.TablePolicyBinding{
			Role:        role,
			Permissions: permissions,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy Binding for ("+role+") in Table ("+plan.Table.ValueString()+"): "+err.Error(),
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
	var state tableIamBindingModel
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

	err := r.config.SpannerService.DeleteTableIamBinding(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instance, database, table),
		role,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting IAM Policy",
			"Could not delete IAM Policy Binding for ("+role+") in Table ("+state.Table.ValueString()+"): "+err.Error(),
		)
		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *tableIamBindingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tableIamBindingResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
