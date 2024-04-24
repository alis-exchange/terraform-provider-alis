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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &databaseIamMemberResource{}
	_ resource.ResourceWithConfigure   = &databaseIamMemberResource{}
	_ resource.ResourceWithImportState = &databaseIamMemberResource{}
)

// NewIamMemberResource is a helper function to simplify the provider implementation.
func NewIamMemberResource() resource.Resource {
	return &databaseIamMemberResource{}
}

type databaseIamMemberResource struct {
}

type databaseIamMemberModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Database types.String `tfsdk:"database"`
	Role     types.String `tfsdk:"role"`
	Member   types.String `tfsdk:"member"`
}

// Metadata returns the resource type name.
func (r *databaseIamMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bigdatabase_database_iam_member"
}

// Schema defines the schema for the resource.
func (r *databaseIamMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
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
			},
			"member": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *databaseIamMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan databaseIamMemberModel
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
	member := plan.Member.ValueString()

	policy, err := services.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: utils.IamPolicyVersion,
		},
	)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			resp.Diagnostics.AddError(
				"Error Reading IAM Policy",
				"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+plan.Database.ValueString()+"): "+err.Error(),
			)
			return
		}

		policy = &iampb.Policy{}
	}

	// Create a map of roles to members
	roleMembersMap := map[string]map[string]string{}
	for _, binding := range policy.GetBindings() {
		if _, ok := roleMembersMap[binding.GetRole()]; !ok {
			roleMembersMap[binding.GetRole()] = map[string]string{}
		}

		for _, m := range binding.GetMembers() {
			roleMembersMap[binding.GetRole()][m] = m
		}
	}

	// Check if the member exists in the role
	roleMembers, roleOk := roleMembersMap[role]
	if !roleOk {
		roleMembers = map[string]string{}
	}
	if _, ok := roleMembers[member]; ok {
		resp.Diagnostics.AddError(
			"Member Already Exists",
			"Member ("+member+") already exists in Role ("+role+") for Database ("+database+")",
		)
		return
	}

	// Add the member to the role
	roleMembers[member] = member
	roleMembersMap[role] = roleMembers

	// Create a new policy
	newPolicy := &iampb.Policy{
		Version:  utils.IamPolicyVersion,
		Bindings: make([]*iampb.Binding, 0),
	}

	// Add the roles to the policy
	for memberRole, members := range roleMembersMap {
		binding := &iampb.Binding{
			Role:    memberRole,
			Members: make([]string, 0),
		}

		for _, m := range members {
			binding.Members = append(binding.Members, m)
		}

		newPolicy.Bindings = append(newPolicy.Bindings, binding)
	}

	_, err = services.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		newPolicy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting IAM Policy",
			"Could not set IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+plan.Database.ValueString()+"): "+err.Error(),
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
func (r *databaseIamMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state databaseIamMemberModel
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
	member := state.Member.ValueString()

	policy, err := services.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: utils.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+database+"): "+err.Error(),
		)
		return
	}

	exists := false

	// Check if the role exists in the policy
	for _, binding := range policy.GetBindings() {
		if binding.GetRole() == role {
			// Check if the member exists in the role
			for _, m := range binding.GetMembers() {
				if m == member {
					exists = true
					return
				}
			}
		}
	}

	if !exists {
		resp.Diagnostics.AddError(
			"Member Not Found",
			"Member ("+member+") not found in Role ("+role+") for Database ("+database+")",
		)
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *databaseIamMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan databaseIamMemberModel
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
	member := plan.Member.ValueString()

	policy, err := services.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: utils.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+plan.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Create a map of roles to members
	roleMembersMap := map[string]map[string]string{}
	for _, binding := range policy.GetBindings() {
		if _, ok := roleMembersMap[binding.GetRole()]; !ok {
			roleMembersMap[binding.GetRole()] = map[string]string{}
		}

		for _, m := range binding.GetMembers() {
			roleMembersMap[binding.GetRole()][m] = m
		}
	}

	// Add the member to the role
	roleMembers, roleOk := roleMembersMap[role]
	if !roleOk {
		roleMembers = map[string]string{}
	}
	if _, ok := roleMembers[member]; !ok {
		roleMembers[member] = member
	}
	roleMembersMap[role] = roleMembers

	// Create a new policy
	newPolicy := &iampb.Policy{
		Version:  utils.IamPolicyVersion,
		Bindings: make([]*iampb.Binding, 0),
	}

	// Add the roles to the policy
	for memberRole, members := range roleMembersMap {
		binding := &iampb.Binding{
			Role:    memberRole,
			Members: make([]string, 0),
		}

		for _, m := range members {
			binding.Members = append(binding.Members, m)
		}

		newPolicy.Bindings = append(newPolicy.Bindings, binding)
	}

	_, err = services.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		newPolicy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting IAM Policy",
			"Could not set IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+plan.Database.ValueString()+"): "+err.Error(),
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

// Delete deletes the resource and removes the Terraform state on success.
func (r *databaseIamMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state databaseIamMemberModel
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
	member := state.Member.ValueString()

	policy, err := services.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: utils.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+database+"): "+err.Error(),
		)
		return
	}

	// Create a map of roles to members
	roleMembersMap := map[string]map[string]string{}
	for _, binding := range policy.GetBindings() {
		if _, ok := roleMembersMap[binding.GetRole()]; !ok {
			roleMembersMap[binding.GetRole()] = map[string]string{}
		}

		for _, m := range binding.GetMembers() {
			roleMembersMap[binding.GetRole()][m] = m
		}
	}

	// Check if member exists in role and delete
	roleMembers, roleOk := roleMembersMap[role]
	if !roleOk {
		roleMembers = map[string]string{}
	}
	if _, ok := roleMembers[member]; ok {
		delete(roleMembers, member)
	}
	roleMembersMap[role] = roleMembers

	// Create a new policy
	newPolicy := &iampb.Policy{
		Version:  utils.IamPolicyVersion,
		Bindings: make([]*iampb.Binding, 0),
	}

	// Add the roles to the policy
	for memberRole, members := range roleMembersMap {
		binding := &iampb.Binding{
			Role:    memberRole,
			Members: make([]string, 0),
		}

		for _, m := range members {
			binding.Members = append(binding.Members, m)
		}

		newPolicy.Bindings = append(newPolicy.Bindings, binding)
	}

	_, err = services.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		newPolicy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting IAM Policy",
			"Could not set IAM Policy for Member ("+member+") in Role ("+role+") in Database ("+state.Database.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *databaseIamMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{database}/roles/{role}/members/{member}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 10 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/roles/{role}/members/{member}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]
	role := importIDParts[7]
	member := importIDParts[9]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), role)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("member"), member)...)
}

// Configure adds the provider configured client to the resource.
func (r *databaseIamMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
}

func (r *databaseIamMemberResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
