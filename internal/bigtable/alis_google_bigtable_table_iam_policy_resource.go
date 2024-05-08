package bigtable

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &tableIamPolicyResource{}
	_ resource.ResourceWithConfigure   = &tableIamPolicyResource{}
	_ resource.ResourceWithImportState = &tableIamPolicyResource{}
)

// NewIamPolicyResource is a helper function to simplify the provider implementation.
func NewIamPolicyResource() resource.Resource {
	return &tableIamPolicyResource{}
}

type tableIamPolicyResource struct {
	config *internal.ProviderConfig
}

// Metadata returns the resource type name.
func (r *tableIamPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_bigtable_table_iam_policy"
}

// Schema defines the schema for the resource.
func (r *tableIamPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"bindings": schema.ListNestedAttribute{
				Required: true,
				CustomType: types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: tableIamPolicyBindingModel{}.attrTypes(),
					},
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Required: true,
							Description: "The role that should be applied. Only one `alis_google_bigtable_table_iam_binding` can be used per role.\n" +
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
				},
				Description: "IAM policy bindings to be set on the table.",
			},
		},
		Description: "Authoritative. Sets the IAM policy for the tables and replaces any existing policy already attached.",
	}
}

// Create a new resource.
func (r *tableIamPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan tableIamPolicyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from plan
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	table := plan.Table.ValueString()

	// Create a new IAM policy
	policy := &iampb.Policy{
		Version:  internal.IamPolicyVersion,
		Bindings: nil,
	}

	// Add bindings to the policy
	if !plan.Bindings.IsNull() {
		bindings := make([]tableIamPolicyBindingModel, 0, len(plan.Bindings.Elements()))
		d := plan.Bindings.ElementsAs(ctx, &bindings, false)
		if d.HasError() {
			resp.Diagnostics.AddError("Error Reading Bindings",
				fmt.Sprintf("Could not read IAM Policy bindings for Table ("+plan.Table.ValueString()+"): %v", d))
			return
		}
		diags.Append(d...)

		for _, binding := range bindings {
			if binding.Role.IsUnknown() || binding.Role.IsNull() {
				continue
			}

			members := make([]string, 0, len(binding.Members))
			for _, member := range binding.Members {
				if member.IsNull() || member.IsUnknown() {
					continue
				}

				members = append(members, member.ValueString())
			}

			role := binding.Role.ValueString()
			policy.Bindings = append(policy.Bindings, &iampb.Binding{
				Role:    role,
				Members: members,
			})
		}
	}

	// Create the IAM policy
	updatedPolicy, err := r.config.BigtableService.SetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table), policy, &fieldmaskpb.FieldMask{Paths: []string{"bindings"}})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating IAM Policy",
			"Could not create IAM Policy for Table ("+plan.Table.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	if updatedPolicy.GetBindings() != nil {
		policyBindings := make([]*tableIamPolicyBindingModel, 0)

		for _, binding := range updatedPolicy.GetBindings() {
			bindingModel := &tableIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: tableIamPolicyBindingModel{}.attrTypes(),
		}, policyBindings)
		diags.Append(d...)

		plan.Bindings = generatedList
	}

	tflog.Error(ctx, fmt.Sprintf("Created IAM Policy for Table (%s)", plan.Table.ValueString()))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *tableIamPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state tableIamPolicyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	table := state.Table.ValueString()

	policy, err := r.config.BigtableService.GetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table), &iampb.GetPolicyOptions{
		RequestedPolicyVersion: internal.IamPolicyVersion,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to get Bigtable Table IAM Policy", err.Error())
		return
	}

	// Map response body to state
	if policy.GetBindings() != nil {
		policyBindings := make([]*tableIamPolicyBindingModel, 0)

		for _, binding := range policy.GetBindings() {
			bindingModel := &tableIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: tableIamPolicyBindingModel{}.attrTypes(),
		}, policyBindings)
		diags.Append(d...)

		state.Bindings = generatedList
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableIamPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan tableIamPolicyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from plan
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	table := plan.Table.ValueString()

	// Create a new IAM policy
	policy := &iampb.Policy{
		Version:  internal.IamPolicyVersion,
		Bindings: nil,
	}

	// Add bindings to the policy
	if !plan.Bindings.IsNull() {
		bindings := make([]tableIamPolicyBindingModel, 0, len(plan.Bindings.Elements()))
		d := plan.Bindings.ElementsAs(ctx, &bindings, false)
		if d.HasError() {
			resp.Diagnostics.AddError("Error Reading Bindings",
				fmt.Sprintf("Could not read IAM Policy bindings for Table ("+plan.Table.ValueString()+"): %v", d))
			return
		}
		diags.Append(d...)

		for _, binding := range bindings {
			if binding.Role.IsUnknown() || binding.Role.IsNull() {
				continue
			}

			members := make([]string, 0, len(binding.Members))
			for _, member := range binding.Members {
				if member.IsNull() || member.IsUnknown() {
					continue
				}

				members = append(members, member.ValueString())
			}

			role := binding.Role.ValueString()
			policy.Bindings = append(policy.Bindings, &iampb.Binding{
				Role:    role,
				Members: members,
			})
		}
	}

	// Create the IAM policy
	updatedPolicy, err := r.config.BigtableService.SetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table), policy, &fieldmaskpb.FieldMask{Paths: []string{"bindings"}})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy for Table ("+plan.Table.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	if updatedPolicy.GetBindings() != nil {
		policyBindings := make([]*tableIamPolicyBindingModel, 0)

		for _, binding := range updatedPolicy.GetBindings() {
			bindingModel := &tableIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: tableIamPolicyBindingModel{}.attrTypes(),
		}, policyBindings)
		diags.Append(d...)

		plan.Bindings = generatedList
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *tableIamPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state tableIamPolicyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and table from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	table := state.Table.ValueString()

	// Create a new IAM policy
	policy := &iampb.Policy{
		Version:  internal.IamPolicyVersion,
		Bindings: nil,
	}

	// Create the IAM policy
	_, err := r.config.BigtableService.SetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table), policy, &fieldmaskpb.FieldMask{Paths: []string{"bindings"}})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting IAM Policy",
			"Could not delete IAM Policy for Table ("+state.Table.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *tableIamPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and table id
	// projects/{project}/instances/{instance}/tables/{table}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 6 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/tables/{table}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	tableName := importIDParts[5]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), tableName)...)
}

// Configure adds the provider configured client to the resource.
func (r *tableIamPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

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

func (r *tableIamPolicyResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
