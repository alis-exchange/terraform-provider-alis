package bigtable

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/bigtable"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"terraform-provider-alis/internal/bigtable/services"
	custom_types "terraform-provider-alis/internal/custom-types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &garbageCollectionPolicyResource{}
	_ resource.ResourceWithConfigure   = &garbageCollectionPolicyResource{}
	_ resource.ResourceWithImportState = &garbageCollectionPolicyResource{}
)

// NewGarbageCollectionPolicyResource is a helper function to simplify the provider implementation.
func NewGarbageCollectionPolicyResource() resource.Resource {
	return &garbageCollectionPolicyResource{}
}

type garbageCollectionPolicyResource struct {
}

type bigtableGarbageCollectionPolicyModel struct {
	Project        types.String                 `tfsdk:"project"`
	Instance       types.String                 `tfsdk:"instance"`
	Table          types.String                 `tfsdk:"table"`
	ColumFamily    types.String                 `tfsdk:"column_family"`
	DeletionPolicy types.String                 `tfsdk:"deletion_policy"`
	GcRules        custom_types.JsonStringValue `tfsdk:"gc_rules"`
}

// Metadata returns the resource type name.
func (r *garbageCollectionPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_bigtable_gc_policy"
}

// Schema defines the schema for the resource.
func (r *garbageCollectionPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"column_family": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "The name of the column family.",
			},
			"deletion_policy": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("ABANDON"),
				},
				Description: "The deletion policy for the GC policy.\n" +
					"Setting `ABANDON` allows the resource to be abandoned rather than deleted.\n" +
					"This is useful for GC policy as it cannot be deleted in a replicated instance.\n" +
					"**Note that when set to `ABANDON`, this only removes the `deletion_policy` from terraform state.\n" +
					"Garbage collection still happens on the specified column family.**\n" +
					"Possible values are: `ABANDON`.",
			},
			"gc_rules": schema.StringAttribute{
				CustomType: custom_types.JsonStringType{},
				Required:   true,
				Description: "Serialized JSON string for garbage collection policy.\n" +
					"Allowed fields:\n" +
					"	- `mode` which can only be `union` or `intersection`.\n" +
					"If not specified then `rules` can only have one JSON object, otherwise must have multiple JSON objects.\n" +
					"	- `rules` which is an array of JSON objects. Each object can only have `max_age` or `max_version`.\n" +
					"Alternatively, you can have nested JSON objects with the same structure as the parent JSON object(`gc_rules`).",
			},
		},
	}
}

// Create a new resource.
func (r *garbageCollectionPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan bigtableGarbageCollectionPolicyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate table from plan
	gcPolicy := bigtable.NoGcPolicy()

	// If gc_rules is set, set gc rules
	if !plan.GcRules.IsNull() {
		gcRules := plan.GcRules.ValueString()
		gcRuleMap := make(map[string]interface{})
		if err := json.Unmarshal([]byte(gcRules), &gcRuleMap); err != nil {
			resp.Diagnostics.AddError(
				"Invalid GC Rules",
				"Could not parse GC Rules: "+err.Error(),
			)
			return
		}

		policy, err := services.GetGcPolicyFromJSON(gcRuleMap, true)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid GC Rules",
				"Could not parse GC Rules: "+err.Error(),
			)
			return
		}

		gcPolicy = policy
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	tableId := plan.Table.ValueString()
	columnFamilyId := plan.ColumFamily.ValueString()

	// Create policy
	createdPolicy, err := services.UpdateBigtableGarbageCollectionPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId), columnFamilyId, &gcPolicy)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating GC Policy",
			"Could not create GC Policy for Table ("+tableId+") and Column Family ("+columnFamilyId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ColumFamily = types.StringValue(columnFamilyId)

	//// Populate deletion policy
	//switch createdPolicy.GetDeletionPolicy() {
	//case pb.BigtableTable_ColumnFamily_GarbageCollectionPolicy_ABANDON:
	//	plan.DeletionPolicy = types.StringValue("ABANDON")
	//}

	// Populate rules
	gcRuleMap, err := services.GcPolicyToGcRuleMap(*createdPolicy, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Parse GC Policy to GC Rule String",
			err.Error(),
		)
		return
	}
	gcRuleBytes, err := json.Marshal(gcRuleMap)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Marshal GC Rule Map to JSON",
			err.Error(),
		)
		return
	}

	plan.GcRules = custom_types.NewJsonStringValue(string(gcRuleBytes))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *garbageCollectionPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state bigtableGarbageCollectionPolicyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.Instance.ValueString()
	tableId := state.Table.ValueString()
	columnFamilyId := state.ColumFamily.ValueString()

	// Read garbage collection policy
	gcPolicy, err := services.GetBigtableGarbageCollectionPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		columnFamilyId,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading GC Policy",
			"Could not read GC Policy for Table ("+tableId+") and Column Family ("+columnFamilyId+"): "+err.Error(),
		)
		return
	}

	// Populate rules
	if gcPolicy != nil {
		gcRuleMap, err := services.GcPolicyToGcRuleMap(*gcPolicy, true)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Parsing GC Policy",
				"Unable to Parse GC Policy to GC Rule String: "+err.Error(),
			)
			return
		}

		gcRuleBytes, err := json.Marshal(gcRuleMap)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Marshalling GC Rule Map to JSON",
				"Unable to Marshal GC Rule Map to JSON: "+err.Error(),
			)
			return
		}

		state.GcRules = custom_types.NewJsonStringValue(string(gcRuleBytes))
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *garbageCollectionPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan bigtableGarbageCollectionPolicyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	tableId := plan.Table.ValueString()
	columnFamilyId := plan.ColumFamily.ValueString()

	// Generate GC Policy from plan
	gcPolicy := bigtable.NoGcPolicy()

	// If gc_rules is set, set gc rules
	if !plan.GcRules.IsNull() {
		gcRules := plan.GcRules.ValueString()
		gcRuleMap := make(map[string]interface{})
		if err := json.Unmarshal([]byte(gcRules), &gcRuleMap); err != nil {
			resp.Diagnostics.AddError(
				"Invalid GC Rules",
				"Could not parse GC Rules: "+err.Error(),
			)
			return
		}

		policy, err := services.GetGcPolicyFromJSON(gcRuleMap, true)
		if err != nil {
			resp.Diagnostics.AddError(
				"Erorr Parsing GC Rules",
				"Could not parse GC Rules: "+err.Error(),
			)
			return
		}

		gcPolicy = policy
	}

	// Update GC Policy
	updatedPolicy, err := services.UpdateBigtableGarbageCollectionPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		columnFamilyId,
		&gcPolicy,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating GC Policy",
			"Could not update GC Policy for ("+columnFamilyId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ColumFamily = types.StringValue(columnFamilyId)

	//// Populate deletion policy
	//switch updatedPolicy.GetDeletionPolicy() {
	//case pb.BigtableTable_ColumnFamily_GarbageCollectionPolicy_ABANDON:
	//	plan.DeletionPolicy = types.StringValue("ABANDON")
	//}
	//
	// Populate rules
	gcRuleMap, err := services.GcPolicyToGcRuleMap(*updatedPolicy, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Parse GC Policy to GC Rule String",
			err.Error(),
		)
		return
	}
	gcRuleBytes, err := json.Marshal(gcRuleMap)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Marshal GC Rule Map to JSON",
			err.Error(),
		)
		return
	}
	plan.GcRules = custom_types.NewJsonStringValue(string(gcRuleBytes))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *garbageCollectionPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state bigtableGarbageCollectionPolicyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If deletion policy is set to ABANDON, return early
	if state.DeletionPolicy.ValueString() == "ABANDON" {
		tflog.Warn(ctx, "Abandoning GC Policy for Table ("+state.Table.ValueString()+") and Column Family ("+state.ColumFamily.ValueString()+")")
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.Instance.ValueString()
	tableName := state.Table.ValueString()
	columnFamilyId := state.ColumFamily.ValueString()

	// Delete existing table
	_, err := services.DeleteBigtableGarbageCollectionPolicy(ctx,
		fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName),
		columnFamilyId,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting GC Policy",
			"Could not delete GC Policy for ("+columnFamilyId+"): "+err.Error(),
		)
		return
	}
}

func (r *garbageCollectionPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// TODO: Refactor
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), tableName)...)
}

// Configure adds the provider configured client to the resource.
func (r *garbageCollectionPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
}
