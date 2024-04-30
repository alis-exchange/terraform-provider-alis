package spanner

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &databaseIamBindingResource{}
	_ resource.ResourceWithConfigure   = &databaseIamBindingResource{}
	_ resource.ResourceWithImportState = &databaseIamBindingResource{}
)

// NewIamBindingResource is a helper function to simplify the provider implementation.
func NewIamBindingResource() resource.Resource {
	return &databaseIamBindingResource{}
}

type databaseIamBindingResource struct {
	config *internal.ProviderConfig
}

type databaseIamBindingModel struct {
	Project  types.String   `tfsdk:"project"`
	Instance types.String   `tfsdk:"instance"`
	Database types.String   `tfsdk:"database"`
	Role     types.String   `tfsdk:"role"`
	Members  []types.String `tfsdk:"members"`
}

// Metadata returns the resource type name.
func (r *databaseIamBindingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database_iam_binding"
}

// Schema defines the schema for the resource.
func (r *databaseIamBindingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"role": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "The role that should be applied. Only one `alis_google_spanner_database_iam_binding` can be used per role.\n" +
					"Note that custom roles must be of the format `[projects|organizations]/{parent-name}/roles/{role-name}`",
			},
			"members": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Identities that will be granted the privilege in `role`. Each entry can have one of the following values:\n" +
					"	- allUsers: A special identifier that represents anyone who is on the internet; with or without a Google account.\n" +
					"	- allAuthenticatedUsers: A special identifier that represents anyone who is authenticated with a Google account or a service account.\n" +
					"	- user:{emailId}: An email address that represents a specific Google account.\n" +
					"	- serviceAccount:{emailId}: An email address that represents a service account.\n" +
					"	- group:{emailId}: An email address that represents a Google group.\n" +
					"	- domain:{domain}: A G Suite domain (primary, instead of alias) name that represents all the users of that domain. For example, google.com or example.com.\n",
			},
		},
		Description: "Authoritative for a given role. Updates the IAM policy to grant a role to a list of members.\n" +
			"Other roles within the IAM policy for the database are preserved.",
	}
}

// Create a new resource.
func (r *databaseIamBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan databaseIamBindingModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	database := plan.Database.ValueString()
	role := plan.Role.ValueString()

	policy, err := r.config.SpannerService.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Database ("+plan.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Iterate over bindings and get other role bindings
	roleMembersMap := map[string][]string{}
	for _, binding := range policy.GetBindings() {
		if role == binding.GetRole() {
			continue
		}

		roleMembersMap[binding.GetRole()] = binding.GetMembers()
	}

	// Add the new role binding if members are provided
	if plan.Members != nil {
		if _, ok := roleMembersMap[role]; !ok {
			roleMembersMap[role] = []string{}
		}

		for _, member := range plan.Members {
			roleMembersMap[role] = append(roleMembersMap[role], member.ValueString())
		}
	}

	// Reset the bindings
	policy.Bindings = make([]*iampb.Binding, 0)
	// Add the bindings to the policy
	for membersRole, members := range roleMembersMap {
		policy.Bindings = append(policy.Bindings, &iampb.Binding{
			Role:    membersRole,
			Members: members,
		})
	}

	_, err = r.config.SpannerService.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		policy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy Binding for ("+role+") in Database ("+plan.Database.ValueString()+"): "+err.Error(),
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
func (r *databaseIamBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state databaseIamBindingModel
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

	policy, err := r.config.SpannerService.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Database ("+state.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to state
	if policy.GetBindings() != nil {
		state.Members = make([]types.String, 0)

		for _, binding := range policy.GetBindings() {
			if role != "" && role != binding.GetRole() {
				continue
			}

			for _, member := range binding.GetMembers() {
				state.Members = append(state.Members, types.StringValue(member))
			}
		}
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *databaseIamBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan databaseIamBindingModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	database := plan.Database.ValueString()
	role := plan.Role.ValueString()

	policy, err := r.config.SpannerService.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Database ("+plan.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Iterate over bindings and get other role bindings
	roleMembersMap := map[string][]string{}
	for _, binding := range policy.GetBindings() {
		if role == binding.GetRole() {
			continue
		}

		roleMembersMap[binding.GetRole()] = binding.GetMembers()
	}

	// Add the new role binding if members are provided
	if plan.Members != nil {
		if _, ok := roleMembersMap[role]; !ok {
			roleMembersMap[role] = []string{}
		}

		for _, member := range plan.Members {
			roleMembersMap[role] = append(roleMembersMap[role], member.ValueString())
		}
	}

	// Reset the bindings
	policy.Bindings = make([]*iampb.Binding, 0)
	// Add the bindings to the policy
	for membersRole, members := range roleMembersMap {
		policy.Bindings = append(policy.Bindings, &iampb.Binding{
			Role:    membersRole,
			Members: members,
		})
	}

	updatedPolicy, err := r.config.SpannerService.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		policy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy Binding for ("+role+") in Database ("+plan.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to state
	if updatedPolicy.GetBindings() != nil {
		plan.Members = make([]types.String, 0)

		for _, binding := range updatedPolicy.GetBindings() {
			if role != "" && role != binding.GetRole() {
				continue
			}

			for _, member := range binding.GetMembers() {
				plan.Members = append(plan.Members, types.StringValue(member))
			}
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
func (r *databaseIamBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state databaseIamBindingModel
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

	policy, err := r.config.SpannerService.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Database ("+state.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Create a map of roles to bindings
	roleBindings := map[string]*iampb.Binding{}
	for _, binding := range policy.GetBindings() {
		if role == binding.GetRole() {
			continue
		}

		roleBindings[binding.GetRole()] = binding
	}

	// Update the IAM policy
	var bindings []*iampb.Binding
	for _, binding := range roleBindings {
		bindings = append(bindings, binding)
	}
	_, err = r.config.SpannerService.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.Policy{Bindings: bindings},
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy Binding for ("+role+") in Database ("+state.Database.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *databaseIamBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{database}/roles/{role}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 8 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/roles/{role}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]
	role := importIDParts[7]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), role)...)
}

// Configure adds the provider configured client to the resource.
func (r *databaseIamBindingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *databaseIamBindingResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
