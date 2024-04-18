package spanner

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
	"terraform-provider-alis/internal/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &spannerTableResource{}
	_ resource.ResourceWithConfigure   = &spannerTableResource{}
	_ resource.ResourceWithImportState = &spannerTableResource{}
)

// NewSpannerTableResource is a helper function to simplify the provider implementation.
func NewSpannerTableResource() resource.Resource {
	return &spannerTableResource{}
}

type spannerTableResource struct {
	client pb.SpannerServiceClient
}

type spannerTableModel struct {
	Name         types.String        `tfsdk:"name"`
	Project      types.String        `tfsdk:"project"`
	InstanceName types.String        `tfsdk:"instance_name"`
	DatabaseName types.String        `tfsdk:"database_name"`
	Schema       *spannerTableSchema `tfsdk:"schema"`
}

type spannerTableSchema struct {
	Columns types.List `tfsdk:"columns"`
	Indices types.List `tfsdk:"indices"`
}

type spannerTableColumn struct {
	Name         types.String `tfsdk:"name"`
	IsPrimaryKey types.Bool   `tfsdk:"is_primary_key"`
	Unique       types.Bool   `tfsdk:"unique"`
	Type         types.String `tfsdk:"type"`
	Size         types.Int64  `tfsdk:"size"`
	Precision    types.Int64  `tfsdk:"precision"`
	Scale        types.Int64  `tfsdk:"scale"`
	Nullable     types.Bool   `tfsdk:"nullable"`
	DefaultValue types.String `tfsdk:"default_value"`
}

func (o spannerTableColumn) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":           types.StringType,
		"is_primary_key": types.BoolType,
		"unique":         types.BoolType,
		"type":           types.StringType,
		"size":           types.Int64Type,
		"precision":      types.Int64Type,
		"scale":          types.Int64Type,
		"nullable":       types.BoolType,
		"default_value":  types.StringType,
	}
}

type spannerTableIndex struct {
	Name    types.String `tfsdk:"name"`
	Columns types.Set    `tfsdk:"columns"`
	Unique  types.Bool   `tfsdk:"unique"`
}

func (o spannerTableIndex) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
		"columns": types.SetType{
			ElemType: types.StringType,
		},
		"unique": types.BoolType,
	}
}

// Metadata returns the resource type name.
func (r *spannerTableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spanner_table"
}

// Schema defines the schema for the resource.
func (r *spannerTableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"database_name": schema.StringAttribute{
				Required: true,
			},
			"schema": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"columns": schema.ListNestedAttribute{
						Required: true,
						CustomType: types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spannerTableColumn{}.attrTypes(),
							},
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Required: true,
								},
								"is_primary_key": schema.BoolAttribute{
									Optional: true,
								},
								"unique": schema.BoolAttribute{
									Optional: true,
								},
								"type": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf(utils.SpannerTableDataTypes...),
									},
								},
								"size": schema.Int64Attribute{
									Optional: true,
								},
								"precision": schema.Int64Attribute{
									Optional: true,
								},
								"scale": schema.Int64Attribute{
									Optional: true,
								},
								"nullable": schema.BoolAttribute{
									Optional: true,
								},
								"default_value": schema.StringAttribute{
									Optional: true,
								},
							},
						},
					},
					"indices": schema.ListNestedAttribute{
						Optional: true,
						CustomType: types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spannerTableIndex{}.attrTypes(),
							},
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Required: true,
								},
								"columns": schema.SetAttribute{
									Required:    true,
									ElementType: types.StringType,
								},
								"unique": schema.BoolAttribute{
									Optional: true,
								},
							},
						},
					},
				},
			},
		},
	}
}

// Create a new resource.
func (r *spannerTableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan spannerTableModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}

	// Generate table from plan
	table := &pb.SpannerTable{
		Name: "",
		Schema: &pb.SpannerTable_Schema{
			Columns:    nil,
			Indices:    nil,
			Interleave: nil,
		},
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.InstanceName.ValueString()
	databaseId := plan.DatabaseName.ValueString()
	tableId := plan.Name.ValueString()

	// Populate schema if any
	if plan.Schema != nil {
		tableSchema := &pb.SpannerTable_Schema{
			Columns:    nil,
			Indices:    nil,
			Interleave: nil,
		}
		if !plan.Schema.Columns.IsNull() {
			columns := make([]*pb.SpannerTable_Schema_Column, 0, len(plan.Schema.Columns.Elements()))
			d := plan.Schema.Columns.ElementsAs(ctx, &columns, false)
			diags.Append(d...)

			tableSchema.Columns = columns
		}
		if !plan.Schema.Indices.IsNull() {
			indices := make([]*pb.SpannerTable_Schema_Index, 0, len(plan.Schema.Indices.Elements()))
			d := plan.Schema.Indices.ElementsAs(ctx, &indices, false)
			diags.Append(d...)

			tableSchema.Indices = indices
		}

		table.Schema = tableSchema
	}

	// Create table
	_, err := r.client.CreateSpannerTable(ctx, &pb.CreateSpannerTableRequest{
		Parent:  fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
		TableId: tableId,
		Table:   table,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Table",
			"Could not create Table ("+plan.Name.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(tableId)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}
}

// Read resource information.
func (r *spannerTableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state spannerTableModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	databaseId := state.DatabaseName.ValueString()
	tableId := state.Name.ValueString()

	// Get table from API
	table, err := r.client.GetSpannerTable(ctx, &pb.GetSpannerTableRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Table",
			"Could not read Table ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Set refreshed state
	state.Name = types.StringValue(tableId)

	// Populate schema
	if table.GetSchema() != nil {
		schema := &spannerTableSchema{}
		if table.GetSchema().GetColumns() != nil {
			columns := make([]*spannerTableColumn, 0)
			for _, column := range table.GetSchema().GetColumns() {
				col := &spannerTableColumn{
					Name:         types.StringValue(column.GetName()),
					IsPrimaryKey: types.BoolValue(column.GetIsPrimaryKey()),
					Unique:       types.BoolValue(column.GetUnique()),
					Type:         types.String{},
					Nullable:     types.BoolValue(column.GetNullable()),
					DefaultValue: types.StringValue(column.GetDefaultValue()),
				}

				// Get type
				switch column.GetType() {
				case pb.SpannerTable_Schema_Column_BOOL:
					col.Type = types.StringValue(utils.SpannerTableDataType_BOOL.String())
				case pb.SpannerTable_Schema_Column_INT64:
					col.Type = types.StringValue(utils.SpannerTableDataType_INT64.String())
				case pb.SpannerTable_Schema_Column_FLOAT64:
					col.Type = types.StringValue(utils.SpannerTableDataType_FLOAT64.String())
				case pb.SpannerTable_Schema_Column_STRING:
					col.Type = types.StringValue(utils.SpannerTableDataType_STRING.String())
				case pb.SpannerTable_Schema_Column_BYTES:
					col.Type = types.StringValue(utils.SpannerTableDataType_BYTES.String())
				case pb.SpannerTable_Schema_Column_DATE:
					col.Type = types.StringValue(utils.SpannerTableDataType_DATE.String())
				case pb.SpannerTable_Schema_Column_TIMESTAMP:
					col.Type = types.StringValue(utils.SpannerTableDataType_TIMESTAMP.String())
				case pb.SpannerTable_Schema_Column_NUMERIC:
					col.Type = types.StringValue(utils.SpannerTableDataType_NUMERIC.String())
				case pb.SpannerTable_Schema_Column_JSON:
					col.Type = types.StringValue(utils.SpannerTableDataType_JSON.String())
				}

				// Get size
				if column.GetSize() != nil {
					col.Size = types.Int64Value(column.GetSize().GetValue())
				}

				// Get precision
				if column.GetPrecision() != nil {
					col.Precision = types.Int64Value(column.GetPrecision().GetValue())
				}

				// Get scale
				if column.GetScale() != nil {
					col.Scale = types.Int64Value(column.GetScale().GetValue())
				}

				columns = append(columns, col)
			}

			generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
				AttrTypes: spannerTableColumn{}.attrTypes(),
			}, columns)
			diags.Append(d...)

			schema.Columns = generatedList
		}
		if table.GetSchema().GetIndices() != nil {
			indices := make([]*spannerTableIndex, 0, len(table.GetSchema().GetIndices()))
			for _, index := range table.GetSchema().GetIndices() {
				idx := &spannerTableIndex{
					Name:    types.StringValue(index.GetName()),
					Columns: types.Set{},
				}

				// Get unique
				if index.GetUnique() != nil {
					idx.Unique = types.BoolValue(index.GetUnique().GetValue())
				}

				// Get columns
				columns := make([]string, 0, len(index.GetColumns()))
				for _, column := range index.GetColumns() {
					columns = append(columns, column)
				}
				generatedSet, d := types.SetValueFrom(ctx, types.StringType, columns)
				diags.Append(d...)

				idx.Columns = generatedSet

				indices = append(indices, idx)
			}

			generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
				AttrTypes: spannerTableIndex{}.attrTypes(),
			}, indices)
			diags.Append(d...)

			schema.Indices = generatedList
		}

		state.Schema = schema
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}
}

func (r *spannerTableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan spannerTableModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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
func (r *spannerTableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state spannerTableModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	databaseId := state.DatabaseName.ValueString()
	tableId := state.Name.ValueString()

	// Delete existing database
	_, err := r.client.DeleteSpannerTable(ctx, &pb.DeleteSpannerTableRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Table",
			"Could not delete Table ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *spannerTableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{table}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 6 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{table}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_name"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), databaseName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerTableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clients.Spanner
}

func (r *spannerTableResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
