package bigtable

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/bigtable/services"
	"terraform-provider-alis/internal/validators"
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
}

type bigtableTableModel struct {
	Name                  types.String `tfsdk:"name"`
	Project               types.String `tfsdk:"project"`
	Instance              types.String `tfsdk:"instance"`
	DeletionProtection    types.Bool   `tfsdk:"deletion_protection"`
	ChangeStreamRetention types.String `tfsdk:"change_stream_retention"`
	ColumnFamilies        types.List   `tfsdk:"column_families"`
}

type bigtableTableColumnFamilyModel struct {
	Name types.String `tfsdk:"name"`
}

func (o bigtableTableColumnFamilyModel) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
	}
}

// Metadata returns the resource type name.
func (r *tableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_bigtable_table"
}

// Schema defines the schema for the resource.
func (r *tableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the table. Must be 1-50 characters and must only contain hyphens, underscores, periods, letters and numbers.",
			},
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID in which the table belongs.",
			},
			"instance": schema.StringAttribute{
				Required:    true,
				Description: "The name of the Bigtable instance.",
			},
			"deletion_protection": schema.BoolAttribute{
				Required: true,
				Description: "A field to make the table protected against data loss i.e. when set to `PROTECTED`, deleting the table,\n" +
					"the column families in the table, and the instance containing the table would be prohibited.\n" +
					"If not provided, currently deletion protection will be set to `UNPROTECTED` as it is the API default value.",
			},
			"change_stream_retention": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[1-9][0-9]*s$`), "Change Stream Retention must be a valid duration specified in seconds in the format `{seconds}s` e.g. 86400s"),
					validators.DurationStringMinSeconds(60 * 60 * 24),
					validators.DurationStringMaxSeconds(60 * 60 * 24 * 7),
				},
				Description: "The duration for which the change stream is retained. The minimum duration is 1 hour and the maximum duration is 7 days. Set to `0s` to disable.",
			},
			"column_families": schema.ListNestedAttribute{
				Optional: true,
				CustomType: types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: bigtableTableColumnFamilyModel{}.attrTypes(),
					},
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the column family.",
						},
					},
				},
				Description: "A group of columns within a table which share a common configuration. This can be specified multiple times.",
			},
		},
		Description: "A Google Bigtable resource.\n" +
			"This resource provisions and manages tables in a Bigtable instance.",
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
	table := &services.BigtableTable{}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	tableId := plan.Name.ValueString()

	// Populate deletion protection if any
	if !plan.DeletionProtection.IsUnknown() && !plan.DeletionProtection.IsNull() {
		if plan.DeletionProtection.ValueBool() {
			table.DeletionProtection = bigtable.Protected
		} else {
			table.DeletionProtection = bigtable.Unprotected
		}
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
		table.ChangeStreamRetention = duration
	}

	// Populate column families if any
	if !plan.ColumnFamilies.IsUnknown() && !plan.ColumnFamilies.IsNull() {
		columnFamilies := make([]bigtableTableColumnFamilyModel, 0, len(plan.ColumnFamilies.Elements()))
		d := plan.ColumnFamilies.ElementsAs(ctx, &columnFamilies, false)
		if d.HasError() {
			resp.Diagnostics.AddError(
				"Error Reading Column Families",
				"Could not read Column Families: ",
			)
			return
		}
		diags.Append(d...)

		if len(columnFamilies) > 0 {
			table.ColumnFamilies = make(map[string]bigtable.Family, len(columnFamilies))
			for _, columnFamily := range columnFamilies {
				if !columnFamily.Name.IsUnknown() && !columnFamily.Name.IsNull() {
					table.ColumnFamilies[columnFamily.Name.ValueString()] = bigtable.Family{}
				}
			}
		}
	}

	// Create table
	_, err := services.CreateBigtableTable(ctx, fmt.Sprintf("projects/%s/instances/%s", project, instanceName), tableId, table)
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
	instanceName := state.Instance.ValueString()
	tableName := state.Name.ValueString()

	tflog.Error(ctx, "Reading Bigtable Table: "+fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName))

	// Get table from API
	table, err := services.GetBigtableTable(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Table",
			"Could not read Table ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Set table id
	state.Name = types.StringValue(tableName)

	// Get deletion protection
	var deletionProtection types.Bool
	// Populate deletion protection if any
	switch table.DeletionProtection {
	case bigtable.None:
		deletionProtection = types.BoolNull()
	case bigtable.Protected:
		deletionProtection = types.BoolValue(true)
	case bigtable.Unprotected:
		deletionProtection = types.BoolValue(false)
	}
	state.DeletionProtection = deletionProtection

	// Get change stream retention
	var changeStreamRetention types.String
	// Populate change stream retention if any
	if table.ChangeStreamRetention != nil {
		duration := table.ChangeStreamRetention.(time.Duration)
		changeStreamRetention = types.StringValue(fmt.Sprintf("%vs", duration.Seconds()))
	} else {
		changeStreamRetention = types.StringNull()
	}
	state.ChangeStreamRetention = changeStreamRetention

	// Populate column families if any
	if table.ColumnFamilies != nil && len(table.ColumnFamilies) > 0 {
		var columnFamiliesList []bigtableTableColumnFamilyModel
		for columnFamilyName := range table.ColumnFamilies {
			// Populate column family
			columnFamiliesList = append(columnFamiliesList, bigtableTableColumnFamilyModel{
				Name: types.StringValue(columnFamilyName),
			})
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: bigtableTableColumnFamilyModel{}.attrTypes(),
		}, columnFamiliesList)
		diags.Append(d...)

		state.ColumnFamilies = generatedList
	}

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
	instanceName := plan.Instance.ValueString()
	tableId := plan.Name.ValueString()

	// Generate table from plan
	table := &services.BigtableTable{
		Name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
	}

	// Populate deletion protection if any
	if !plan.DeletionProtection.IsUnknown() && !plan.DeletionProtection.IsNull() {
		if plan.DeletionProtection.ValueBool() {
			table.DeletionProtection = bigtable.Protected
		} else {
			table.DeletionProtection = bigtable.Unprotected
		}
	} else {
		table.DeletionProtection = bigtable.None
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
		table.ChangeStreamRetention = duration
	}

	// Populate column families if any
	if !plan.ColumnFamilies.IsUnknown() && !plan.ColumnFamilies.IsNull() {
		columnFamilies := make([]bigtableTableColumnFamilyModel, 0, len(plan.ColumnFamilies.Elements()))
		d := plan.ColumnFamilies.ElementsAs(ctx, &columnFamilies, false)
		if d.HasError() {
			resp.Diagnostics.AddError(
				"Error Reading Column Families",
				"Could not read Column Families: ",
			)
			return
		}
		diags.Append(d...)

		if len(columnFamilies) > 0 {
			table.ColumnFamilies = make(map[string]bigtable.Family, len(columnFamilies))
			for _, columnFamily := range columnFamilies {
				if !columnFamily.Name.IsUnknown() && !columnFamily.Name.IsNull() {
					table.ColumnFamilies[columnFamily.Name.ValueString()] = bigtable.Family{}
				}
			}
		}
	}

	// Update existing table
	_, err := services.UpdateBigtableTable(ctx, table, &fieldmaskpb.FieldMask{
		Paths: []string{"deletion_protection", "change_stream_retention", "column_families"},
	}, true)
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
	instanceName := state.Instance.ValueString()
	tableName := state.Name.ValueString()

	// Delete existing table
	_, err := services.DeleteBigtableTable(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableName))
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), tableName)...)
}

// Configure adds the provider configured client to the resource.
func (r *tableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	//clients, ok := req.ProviderData.(utils.ProviderClients)
	//if !ok {
	//	resp.Diagnostics.AddError(
	//		"Unexpected Data Source Configure Type",
	//		fmt.Sprintf("Expected ProviderClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
	//	)
	//
	//	return
	//}
	//
	//r.client = clients.Bigtable
}

func (r *tableResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
