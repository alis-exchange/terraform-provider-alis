package bigtable

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
	alUtils "go.alis.build/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &tableIamMemberResource{}
	_ resource.ResourceWithConfigure   = &tableIamMemberResource{}
	_ resource.ResourceWithImportState = &tableIamMemberResource{}
)

// NewIamMemberResource is a helper function to simplify the provider implementation.
func NewIamMemberResource() resource.Resource {
	return &tableIamMemberResource{}
}

type tableIamMemberResource struct {
	config *internal.ProviderConfig
}

type tableIamMemberModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Table    types.String `tfsdk:"table"`
	Role     types.String `tfsdk:"role"`
	Member   types.String `tfsdk:"member"`
}

// Metadata returns the resource type name.
func (r *tableIamMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_bigtable_table_iam_member"
}

// Schema defines the schema for the resource.
func (r *tableIamMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID.",
			},
			"instance": schema.StringAttribute{
				Required:    true,
				Description: "The Bigtable instance ID.",
			},
			"table": schema.StringAttribute{
				Required:    true,
				Description: "The Bigtable table ID.",
			},
			"role": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "The role that should be applied.\n" +
					"Note that custom roles must be of the format `[projects|organizations]/{parent-name}/roles/{role-name}`",
			},
			"member": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "Identity that will be granted the privilege in `role`. Can have one of the following values:\n" +
					"	- allUsers: A special identifier that represents anyone who is on the internet; with or without a Google account.\n" +
					"	- allAuthenticatedUsers: A special identifier that represents anyone who is authenticated with a Google account or a service account.\n" +
					"	- user:{emailId}: An email address that represents a specific Google account.\n" +
					"	- serviceAccount:{emailId}: An email address that represents a service account.\n" +
					"	- group:{emailId}: An email address that represents a Google group.\n" +
					"	- domain:{domain}: A G Suite domain (primary, instead of alias) name that represents all the users of that domain. For example, google.com or example.com.\n",
			},
		},
		Description:        "Non-authoritative. Updates the IAM policy to grant a role to a new member. Other members for the role for the table are preserved.",
		DeprecationMessage: "This resource is deprecated. Please use the standard Google provider resource instead.",
	}
}

// Create a new resource.
func (r *tableIamMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan tableIamMemberModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	table := plan.Table.ValueString()
	role := plan.Role.ValueString()
	member := plan.Member.ValueString()

	policy, err := r.config.BigtableService.GetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			resp.Diagnostics.AddError(
				"Error Reading IAM Policy",
				"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+plan.Table.ValueString()+"): "+err.Error(),
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
			"Member ("+member+") already exists in Role ("+role+") for Table ("+table+")",
		)
		return
	}

	// Add the member to the role
	roleMembers[member] = member
	roleMembersMap[role] = roleMembers

	// Create a new policy
	newPolicy := &iampb.Policy{
		Version:  internal.IamPolicyVersion,
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

	_, err = r.config.BigtableService.SetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		newPolicy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting IAM Policy",
			"Could not set IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+plan.Table.ValueString()+"): "+err.Error(),
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
func (r *tableIamMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state tableIamMemberModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	table := state.Table.ValueString()
	role := state.Role.ValueString()
	member := state.Member.ValueString()

	policy, err := r.config.BigtableService.GetBigtableTableIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+table+"): "+err.Error(),
		)
		return
	}

	// Check if the role exists in the policy
	roleBinding, _, found := alUtils.Find(policy.GetBindings(), func(binding *iampb.Binding) bool {
		return binding.GetRole() == role
	})
	if !found || roleBinding == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if !alUtils.Contains(roleBinding.GetMembers(), member) {
		resp.State.RemoveResource(ctx)
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableIamMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan tableIamMemberModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	table := plan.Table.ValueString()
	role := plan.Role.ValueString()
	member := plan.Member.ValueString()

	policy, err := r.config.BigtableService.GetBigtableTableIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+plan.Table.ValueString()+"): "+err.Error(),
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
		Version:  internal.IamPolicyVersion,
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

	_, err = r.config.BigtableService.SetBigtableTableIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		newPolicy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting IAM Policy",
			"Could not set IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+plan.Table.ValueString()+"): "+err.Error(),
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
func (r *tableIamMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state tableIamMemberModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	table := state.Table.ValueString()
	role := state.Role.ValueString()
	member := state.Member.ValueString()

	policy, err := r.config.BigtableService.GetBigtableTableIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: internal.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Policy",
			"Could not read IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+table+"): "+err.Error(),
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
		Version:  internal.IamPolicyVersion,
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

	_, err = r.config.BigtableService.SetBigtableTableIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		newPolicy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting IAM Policy",
			"Could not set IAM Policy for Member ("+member+") in Role ("+role+") in Table ("+state.Table.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *tableIamMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and table id
	// projects/{project}/instances/{instance}/tables/{table}/roles/{role}/members/{member}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 10 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/tables/{table}/roles/{role}/members/{member}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	tableName := importIDParts[5]
	role := importIDParts[7]
	member := importIDParts[9]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), tableName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), role)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("member"), member)...)
}

// Configure adds the provider configured client to the resource.
func (r *tableIamMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tableIamMemberResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
