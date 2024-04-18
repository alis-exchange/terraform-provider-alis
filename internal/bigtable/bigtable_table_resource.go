package bigtable

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &tableResource{}
	_ resource.ResourceWithConfigure   = &tableResource{}
	_ resource.ResourceWithImportState = &tableResource{}
)

// NewTableResource is a helper function to simplify the provider implementation.
func NewTableResource() resource.Resource {
	return &tableResource{}
}

type tableResource struct {
	client pb.BigtableServiceClient
}

type bigtableTableModel struct {
	Name                  types.String                     `tfsdk:"name"`
	Project               types.String                     `tfsdk:"project"`
	InstanceName          types.String                     `tfsdk:"instance_name"`
	SplitKeys             []types.String                   `tfsdk:"split_keys"`
	DeletionProtection    types.Bool                       `tfsdk:"deletion_protection"`
	ChangeStreamRetention types.String                     `tfsdk:"change_stream_retention"`
	ColumnFamilies        []bigtableTableColumnFamilyModel `tfsdk:"column_families"`
}

type bigtableTableColumnFamilyModel struct {
	Name types.String `tfsdk:"name"`
}

// Metadata returns the resource type name.
func (r *tableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bigtable_table"
}

// Schema defines the schema for the resource.
func (r *tableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"project": schema.StringAttribute{
				Required: true,
			},
			"instance_name": schema.StringAttribute{
				Required: true,
			},
			"split_keys": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"deletion_protection": schema.BoolAttribute{
				Optional: true,
			},
			"change_stream_retention": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[1-9][0-9]*s$`), "Version Retention Period must be a valid duration specified in seconds in the format `{seconds}s` e.g. 60s"),
				},
			},
			"column_families": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
		},
	}
}

// Create a new resource.
func (r *tableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan bigtableTableModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate table from plan
	table := &pb.BigtableTable{
		SplitKeys:      make([]string, 0),
		ColumnFamilies: make(map[string]*pb.BigtableTable_ColumnFamily),
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.InstanceName.ValueString()
	tableId := plan.Name.ValueString()

	// Populate split keys if any
	if plan.SplitKeys != nil && len(plan.SplitKeys) > 0 {
		for _, splitKey := range plan.SplitKeys {
			table.SplitKeys = append(table.SplitKeys, splitKey.ValueString())
		}
	}

	// Populate deletion protection if any
	if !plan.DeletionProtection.IsNull() {
		if plan.DeletionProtection.ValueBool() {
			table.DeletionProtection = pb.BigtableTable_PROTECTED
		} else {
			table.DeletionProtection = pb.BigtableTable_UNPROTECTED
		}
	} else {
		table.DeletionProtection = pb.BigtableTable_DELETION_PROTECTION_UNSPECIFIED
	}

	// Populate change stream retention if any
	if !plan.ChangeStreamRetention.IsNull() {
		duration, err := time.ParseDuration(plan.ChangeStreamRetention.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Parsing Change Stream Retention",
				"Could not parse Change Stream Retention: "+err.Error(),
			)
			return
		}
		table.ChangeStreamRetention = durationpb.New(duration)
	}

	// Populate column families if any
	if plan.ColumnFamilies != nil && len(plan.ColumnFamilies) > 0 {
		for _, columnFamily := range plan.ColumnFamilies {
			// Populate column family
			table.ColumnFamilies[columnFamily.Name.ValueString()] = &pb.BigtableTable_ColumnFamily{
				Name: columnFamily.Name.ValueString(),
			}
		}
	}

	// Create table
	_, err := r.client.CreateBigtableTable(ctx, &pb.CreateBigtableTableRequest{
		Parent:  fmt.Sprintf("projects/%s/instances/%s", project, instanceName),
		TableId: tableId,
		Table:   table,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Table",
			"Could not create Table ("+tableId+") in project ("+project+") and instance ("+instanceName+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(tableId)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *tableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state bigtableTableModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	tableName := state.Name.ValueString()

	// Get table from API
	table, err := r.client.GetBigtableTable(ctx, &pb.GetBigtableTableRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Table",
			"Could not read Table ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Set table id
	state.Name = types.StringValue(tableName)

	// Get split keys
	var splitKeys []types.String
	// Populate split keys if any
	if table.GetSplitKeys() != nil && len(table.GetSplitKeys()) > 0 {
		for _, splitKey := range table.GetSplitKeys() {
			splitKeys = append(splitKeys, types.StringValue(splitKey))
		}
	}
	state.SplitKeys = splitKeys

	// Get deletion protection
	var deletionProtection types.Bool
	// Populate deletion protection if any
	switch table.GetDeletionProtection() {
	case pb.BigtableTable_DELETION_PROTECTION_UNSPECIFIED:
		deletionProtection = types.BoolValue(false)
	case pb.BigtableTable_PROTECTED:
		deletionProtection = types.BoolValue(true)
	case pb.BigtableTable_UNPROTECTED:
		deletionProtection = types.BoolValue(false)
	}
	state.DeletionProtection = deletionProtection

	// Get change stream retention
	var changeStreamRetention types.String
	// Populate change stream retention if any
	if table.GetChangeStreamRetention() != nil {
		changeStreamRetention = types.StringValue(table.GetChangeStreamRetention().AsDuration().String())
	} else {
		changeStreamRetention = types.StringValue("0s")
	}
	state.ChangeStreamRetention = changeStreamRetention

	// Get column families
	var columnFamilies []bigtableTableColumnFamilyModel
	// Populate column families if any
	if table.GetColumnFamilies() != nil && len(table.GetColumnFamilies()) > 0 {
		columnFamilies = make([]bigtableTableColumnFamilyModel, 0)
		for columnFamilyName := range table.GetColumnFamilies() {
			// Populate column family
			columnFamilies = append(columnFamilies, bigtableTableColumnFamilyModel{
				Name: types.StringValue(columnFamilyName),
			})
		}
	}
	state.ColumnFamilies = columnFamilies

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan bigtableTableModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.InstanceName.ValueString()
	tableId := plan.Name.ValueString()

	// Generate table from plan
	table := &pb.BigtableTable{
		Name:           fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		SplitKeys:      make([]string, 0),
		ColumnFamilies: make(map[string]*pb.BigtableTable_ColumnFamily),
	}

	// Populate split keys if any
	if plan.SplitKeys != nil && len(plan.SplitKeys) > 0 {
		for _, splitKey := range plan.SplitKeys {
			table.SplitKeys = append(table.SplitKeys, splitKey.ValueString())
		}
	}

	// Populate deletion protection if any
	if !plan.DeletionProtection.IsNull() {
		if plan.DeletionProtection.ValueBool() {
			table.DeletionProtection = pb.BigtableTable_PROTECTED
		} else {
			table.DeletionProtection = pb.BigtableTable_UNPROTECTED
		}
	} else {
		table.DeletionProtection = pb.BigtableTable_DELETION_PROTECTION_UNSPECIFIED
	}

	// Populate change stream retention if any
	if !plan.ChangeStreamRetention.IsNull() {
		duration, err := time.ParseDuration(plan.ChangeStreamRetention.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Parsing Change Stream Retention",
				"Could not parse Change Stream Retention: "+err.Error(),
			)
			return
		}
		table.ChangeStreamRetention = durationpb.New(duration)
	}

	// Populate column families if any
	if plan.ColumnFamilies != nil && len(plan.ColumnFamilies) > 0 {
		for _, columnFamily := range plan.ColumnFamilies {
			// Populate column family
			table.ColumnFamilies[columnFamily.Name.ValueString()] = &pb.BigtableTable_ColumnFamily{
				Name: columnFamily.Name.ValueString(),
			}
		}
	}

	// Update existing table
	_, err := r.client.UpdateBigtableTable(ctx, &pb.UpdateBigtableTableRequest{
		Table: table,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"split_keys", "deletion_protection", "change_stream_retention", "column_families"},
		},
		AllowMissing: true,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Table",
			"Could not update Table ("+tableId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(tableId)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *tableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state bigtableTableModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	tableName := state.Name.ValueString()

	// Delete existing table
	_, err := r.client.DeleteBigtableTable(ctx, &pb.DeleteBigtableTableRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Table",
			"Could not delete Table ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *tableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
func (r *tableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tableResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
