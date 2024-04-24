package spanner

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
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &databaseIamPolicyResource{}
	_ resource.ResourceWithConfigure   = &databaseIamPolicyResource{}
	_ resource.ResourceWithImportState = &databaseIamPolicyResource{}
)

// NewIamPolicyResource is a helper function to simplify the provider implementation.
func NewIamPolicyResource() resource.Resource {
	return &databaseIamPolicyResource{}
}

type databaseIamPolicyResource struct {
}

// Metadata returns the resource type name.
func (r *databaseIamPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spanner_database_iam_policy"
}

// Schema defines the schema for the resource.
func (r *databaseIamPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"bindings": schema.ListNestedAttribute{
				Required: true,
				CustomType: types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: databaseIamPolicyBindingModel{}.attrTypes(),
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
func (r *databaseIamPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan databaseIamPolicyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from plan
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	database := plan.Database.ValueString()

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
	updatedPolicy, err := services.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		policy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating IAM Policy",
			"Could not create IAM Policy for Database ("+plan.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	if updatedPolicy.GetBindings() != nil {
		policyBindings := make([]*databaseIamPolicyBindingModel, 0)

		for _, binding := range updatedPolicy.GetBindings() {
			bindingModel := &databaseIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: databaseIamPolicyBindingModel{}.attrTypes(),
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
func (r *databaseIamPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state databaseIamPolicyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()

	policy, err := services.GetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		&iampb.GetPolicyOptions{
			RequestedPolicyVersion: utils.IamPolicyVersion,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Spanner Database IAM Policy", err.Error())
		return
	}

	// Map response body to state
	if policy.GetBindings() != nil {
		policyBindings := make([]*databaseIamPolicyBindingModel, 0)

		for _, binding := range policy.GetBindings() {
			bindingModel := &databaseIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: databaseIamPolicyBindingModel{}.attrTypes(),
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

func (r *databaseIamPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan databaseIamPolicyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from plan
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	database := plan.Database.ValueString()

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
	updatedPolicy, err := services.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		policy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating IAM Policy",
			"Could not update IAM Policy for Database ("+plan.Database.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	if updatedPolicy.GetBindings() != nil {
		policyBindings := make([]*databaseIamPolicyBindingModel, 0)

		for _, binding := range updatedPolicy.GetBindings() {
			bindingModel := &databaseIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: databaseIamPolicyBindingModel{}.attrTypes(),
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
func (r *databaseIamPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state databaseIamPolicyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()

	// Create a new IAM policy
	policy := &iampb.Policy{
		Version:  utils.IamPolicyVersion,
		Bindings: nil,
	}

	// Create the IAM policy
	_, err := services.SetSpannerDatabaseIamPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database),
		policy,
		&fieldmaskpb.FieldMask{Paths: []string{"bindings"}},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting IAM Policy",
			"Could not delete IAM Policy for Database ("+state.Database.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *databaseIamPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{database}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 6 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
}

// Configure adds the provider configured client to the resource.
func (r *databaseIamPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
}

func (r *databaseIamPolicyResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
