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
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/utils"
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
	client pb.BigtableServiceClient
}

// Metadata returns the resource type name.
func (r *tableIamPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bigtable_table_iam_policy"
}

// Schema defines the schema for the resource.
func (r *tableIamPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"table": schema.StringAttribute{
				Required: true,
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
						},
						"members": schema.ListAttribute{
							ElementType: types.StringType,
							Required:    true,
						},
					},
				},
			},
		},
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
		Version:  utils.IamPolicyVersion,
		Bindings: nil,
	}

	// Add bindings to the policy
	if !plan.Bindings.IsNull() {
		bindings := make([]*iampb.Binding, 0)
		d := plan.Bindings.ElementsAs(ctx, &bindings, false)
		if d.HasError() {
			tflog.Error(ctx, fmt.Sprintf("Error reading bindings: %v", d))
			return
		}
		diags.Append(d...)

		policy.Bindings = bindings
	}

	// Create the IAM policy
	updatedPolicy, err := r.client.SetBigtableTableIamPolicy(ctx, &pb.SetBigtableTableIamPolicyRequest{
		Parent:     fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		Policy:     policy,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	})
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

	policy, err := r.client.GetBigtableTableIamPolicy(ctx, &pb.GetBigtableTableIamPolicyRequest{
		Parent: fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		Options: &pb.GetBigtableTableIamPolicyRequest_GetPolicyOptions{
			RequestedPolicyVersion: utils.IamPolicyVersion,
		},
	})
	if err != nil {
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
		Version:  utils.IamPolicyVersion,
		Bindings: nil,
	}

	// Add bindings to the policy
	if !plan.Bindings.IsNull() {
		bindings := make([]*iampb.Binding, 0)
		d := plan.Bindings.ElementsAs(ctx, &bindings, false)
		if d.HasError() {
			tflog.Error(ctx, fmt.Sprintf("Error reading bindings: %v", d))
			return
		}
		diags.Append(d...)

		policy.Bindings = bindings
	}

	// Create the IAM policy
	updatedPolicy, err := r.client.SetBigtableTableIamPolicy(ctx, &pb.SetBigtableTableIamPolicyRequest{
		Parent:     fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		Policy:     policy,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	})
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
		Version:  utils.IamPolicyVersion,
		Bindings: nil,
	}

	// Create the IAM policy
	_, err := r.client.SetBigtableTableIamPolicy(ctx, &pb.SetBigtableTableIamPolicyRequest{
		Parent:     fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table),
		Policy:     policy,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	})
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

	clients, ok := req.ProviderData.(utils.ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected ProviderClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = clients.Bigtable
}

func (r *tableIamPolicyResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
