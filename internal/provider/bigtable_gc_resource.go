package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	GCPolicyModeIntersection = "INTERSECTION"
	GCPolicyModeUnion        = "UNION"
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
	client pb.BigtableServiceClient
}

// Metadata returns the resource type name.
func (r *garbageCollectionPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bigtable_gc_policy"
}

// Schema defines the schema for the resource.
func (r *garbageCollectionPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required: true,
			},
			"instance_name": schema.StringAttribute{
				Required: true,
			},
			"table": schema.StringAttribute{
				Required: true,
			},
			"column_family": schema.StringAttribute{
				Required: true,
			},
			"deletion_policy": schema.StringAttribute{
				Optional: true,
			},
			"gc_rules": schema.StringAttribute{
				Required: true,
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
	gcPolicy := &pb.Table_ColumnFamily_GarbageCollectionPolicy{
		GcRule: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule{},
	}

	// Set deletion policy
	if !plan.DeletionPolicy.IsNull() {
		if plan.DeletionPolicy.ValueString() == "ABANDON" {
			gcPolicy.DeletionPolicy = pb.Table_ColumnFamily_GarbageCollectionPolicy_ABANDON
		}
	}

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

		gcRule, err := getGCPolicyFromJSON(gcRuleMap, true)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid GC Rules",
				"Could not parse GC Rules: "+err.Error(),
			)
			return
		}

		gcPolicy.GcRule = gcRule
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.InstanceName.ValueString()
	tableId := plan.Table.ValueString()
	columnFamilyId := plan.ColumFamily.ValueString()

	// Create table
	_, err := r.client.UpdateGarbageCollectionPolicy(ctx, &pb.UpdateGarbageCollectionPolicyRequest{
		Parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		ColumnFamilyId: columnFamilyId,
		GcPolicy:       gcPolicy,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating GC Policy",
			"Could not create GC Policy for ("+columnFamilyId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ColumFamily = types.StringValue(columnFamilyId)

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
	instanceName := state.InstanceName.ValueString()
	tableId := state.Table.ValueString()
	columnFamilyId := state.ColumFamily.ValueString()

	// Read garbage collection policy
	gcPolicy, err := r.client.GetGarbageCollectionPolicy(ctx, &pb.GetGarbageCollectionPolicyRequest{
		Parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		ColumnFamilyId: columnFamilyId,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read DB Tables",
			err.Error(),
		)
		return
	}

	// Populate deletion policy
	switch gcPolicy.GetDeletionPolicy() {
	case pb.Table_ColumnFamily_GarbageCollectionPolicy_ABANDON:
		state.DeletionPolicy = types.StringValue("ABANDON")
	}

	// Populate rules
	if gcPolicy.GetGcRule() != nil {
		gcRuleMap, err := GcPolicyToGCRuleMap(gcPolicy.GetGcRule(), true)
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

		state.GcRules = types.StringValue(string(gcRuleBytes))
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
	instanceName := plan.InstanceName.ValueString()
	tableId := plan.Table.ValueString()
	columnFamilyId := plan.ColumFamily.ValueString()

	// Generate GC Policy from plan
	gcPolicy := &pb.Table_ColumnFamily_GarbageCollectionPolicy{
		GcRule: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule{},
	}

	// Set deletion policy
	if !plan.DeletionPolicy.IsNull() {
		if plan.DeletionPolicy.ValueString() == "ABANDON" {
			gcPolicy.DeletionPolicy = pb.Table_ColumnFamily_GarbageCollectionPolicy_ABANDON
		}
	}

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

		gcRule, err := getGCPolicyFromJSON(gcRuleMap, true)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid GC Rules",
				"Could not parse GC Rules: "+err.Error(),
			)
			return
		}

		gcPolicy.GcRule = gcRule
	}

	// Update GC Policy
	_, err := r.client.UpdateGarbageCollectionPolicy(ctx, &pb.UpdateGarbageCollectionPolicyRequest{
		Parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		ColumnFamilyId: columnFamilyId,
		GcPolicy:       gcPolicy,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"gc_rule", "deletion_policy"},
		},
		AllowMissing: true,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating GC Policy",
			"Could not update GC Policy for ("+columnFamilyId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ColumFamily = types.StringValue(columnFamilyId)

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

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	tableName := state.Table.ValueString()
	columnFamilyId := state.ColumFamily.ValueString()

	// Delete existing table
	_, err := r.client.DeleteGarbageCollectionPolicy(ctx, &pb.DeleteGarbageCollectionPolicyRequest{
		Parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName),
		ColumnFamilyId: columnFamilyId,
	})
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_name"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), tableName)...)
}

// Configure adds the provider configured client to the resource.
func (r *garbageCollectionPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(pb.BigtableServiceClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected pb.BigtableServiceClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Recursively convert Bigtable GC policy to JSON format in a map.
func GcPolicyToGCRuleMap(gcRule *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule, topLevel bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	switch gcRule.GetRule().(type) {
	case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_MaxNumVersions:
		// bigtable.MaxVersionsGCPolicy is an int.
		// Not sure why max_version is a float64.
		// TODO: Maybe change max_version to an int.
		version := float64(gcRule.GetMaxNumVersions())
		if topLevel {
			rule := make(map[string]interface{})
			rule["max_version"] = version
			rules := []interface{}{}
			rules = append(rules, rule)
			result["rules"] = rules
		} else {
			result["max_version"] = version
		}
		break
	case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_MaxAge:
		if gcRule.GetMaxAge() != nil {
			age := gcRule.GetMaxAge().AsDuration().String()
			if topLevel {
				rule := make(map[string]interface{})
				rule["max_age"] = age
				rules := []interface{}{}
				rules = append(rules, rule)
				result["rules"] = rules
			} else {
				result["max_age"] = age
			}
			break
		}
	case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Union_:
		result["mode"] = "union"
		rules := []interface{}{}
		for _, r := range gcRule.GetUnion().GetRules() {
			gcRuleString, err := GcPolicyToGCRuleMap(r, false)
			if err != nil {
				return nil, err
			}
			rules = append(rules, gcRuleString)
		}
		result["rules"] = rules
		break
	case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Intersection_:
		result["mode"] = "intersection"
		rules := []interface{}{}
		for _, r := range gcRule.GetIntersection().GetRules() {
			gcRuleString, err := GcPolicyToGCRuleMap(r, false)
			if err != nil {
				return nil, err
			}
			rules = append(rules, gcRuleString)
		}
		result["rules"] = rules
	default:
		break
	}

	if err := validateNestedPolicy(result, topLevel); err != nil {
		return nil, err
	}

	return result, nil
}

func getGCPolicyFromJSON(inputPolicy map[string]interface{}, isTopLevel bool) (*pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule, error) {
	policy := make([]*pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule, 0)

	if err := validateNestedPolicy(inputPolicy, isTopLevel); err != nil {
		return nil, err
	}

	for _, p := range inputPolicy["rules"].([]interface{}) {
		childPolicy := p.(map[string]interface{})
		if err := validateNestedPolicy(childPolicy /*isTopLevel=*/, false); err != nil {
			return nil, err
		}

		if childPolicy["max_age"] != nil {
			maxAge := childPolicy["max_age"].(string)
			duration, err := time.ParseDuration(maxAge)
			if err != nil {
				return nil, fmt.Errorf("invalid duration string: %v", maxAge)
			}

			policy = append(policy, &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule{
				Rule: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_MaxAge{
					MaxAge: durationpb.New(duration),
				},
			})
		}

		if childPolicy["max_version"] != nil {
			version := childPolicy["max_version"].(float64)

			policy = append(policy, &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule{
				Rule: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_MaxNumVersions{
					MaxNumVersions: int32(version),
				},
			})
		}

		if childPolicy["mode"] != nil {
			n, err := getGCPolicyFromJSON(childPolicy /*isTopLevel=*/, false)
			if err != nil {
				return nil, err
			}
			policy = append(policy, n)
		}
	}

	switch inputPolicy["mode"] {
	case strings.ToLower(GCPolicyModeUnion):
		return &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule{
			Rule: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Union_{
				Union: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Union{
					Rules: policy,
				},
			},
		}, nil
	case strings.ToLower(GCPolicyModeIntersection):
		return &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule{
			Rule: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Intersection_{
				Intersection: &pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Intersection{
					Rules: policy,
				},
			},
		}, nil
	default:
		return policy[0], nil
	}
}

func validateNestedPolicy(p map[string]interface{}, isTopLevel bool) error {
	if len(p) > 2 {
		return fmt.Errorf("rules has more than 2 fields")
	}
	maxVersion, maxVersionOk := p["max_version"]
	maxAge, maxAgeOk := p["max_age"]
	rulesObj, rulesOk := p["rules"]

	_, modeOk := p["mode"]
	rules, arrOk := rulesObj.([]interface{})
	_, vCastOk := maxVersion.(float64)
	_, aCastOk := maxAge.(string)

	if rulesOk && !arrOk {
		return fmt.Errorf("`rules` must be array")
	}

	if modeOk && len(rules) < 2 {
		return fmt.Errorf("`rules` need at least 2 GC rule when mode is specified")
	}

	if isTopLevel && !rulesOk {
		return fmt.Errorf("invalid nested policy, need `rules`")
	}

	if isTopLevel && !modeOk && len(rules) != 1 {
		return fmt.Errorf("when `mode` is not specified, `rules` can only have 1 child rule")
	}

	if !isTopLevel && len(p) == 2 && (!modeOk || !rulesOk) {
		return fmt.Errorf("need `mode` and `rules` for child nested policies")
	}

	if !isTopLevel && len(p) == 1 && !maxVersionOk && !maxAgeOk {
		return fmt.Errorf("need `max_version` or `max_age` for the rule")
	}

	if maxVersionOk && !vCastOk {
		return fmt.Errorf("`max_version` must be a number")
	}

	if maxAgeOk && !aCastOk {
		return fmt.Errorf("`max_age must be a string")
	}

	return nil
}
