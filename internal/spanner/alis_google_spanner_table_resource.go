package spanner

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"
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
	config *internal.ProviderConfig
}

type spannerTableModel struct {
	Name     types.String        `tfsdk:"name"`
	Project  types.String        `tfsdk:"project"`
	Instance types.String        `tfsdk:"instance"`
	Database types.String        `tfsdk:"database"`
	Schema   *spannerTableSchema `tfsdk:"schema"`
}

type spannerTableSchema struct {
	Columns types.List `tfsdk:"columns"`
	Indices types.List `tfsdk:"indices"`
}

type spannerTableColumn struct {
	Name          types.String `tfsdk:"name"`
	IsPrimaryKey  types.Bool   `tfsdk:"is_primary_key"`
	AutoIncrement types.Bool   `tfsdk:"auto_increment"`
	Unique        types.Bool   `tfsdk:"unique"`
	Type          types.String `tfsdk:"type"`
	Size          types.Int64  `tfsdk:"size"`
	Precision     types.Int64  `tfsdk:"precision"`
	Scale         types.Int64  `tfsdk:"scale"`
	Required      types.Bool   `tfsdk:"required"`
	DefaultValue  types.String `tfsdk:"default_value"`
}

func (o spannerTableColumn) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":           types.StringType,
		"is_primary_key": types.BoolType,
		"auto_increment": types.BoolType,
		"unique":         types.BoolType,
		"type":           types.StringType,
		"size":           types.Int64Type,
		"precision":      types.Int64Type,
		"scale":          types.Int64Type,
		"required":       types.BoolType,
		"default_value":  types.StringType,
	}
}

type spannerTableIndex struct {
	Name    types.String `tfsdk:"name"`
	Columns types.List   `tfsdk:"columns"`
	Unique  types.Bool   `tfsdk:"unique"`
}

func (o spannerTableIndex) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
		"columns": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":  types.StringType,
					"order": types.StringType,
				},
			},
		},
		"unique": types.BoolType,
	}
}

type spannerTableIndexColumn struct {
	Name  types.String `tfsdk:"name"`
	Order types.String `tfsdk:"order"`
}

func (o spannerTableIndexColumn) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":  types.StringType,
		"order": types.StringType,
	}
}

// Metadata returns the resource type name.
func (r *spannerTableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table"
}

// Schema defines the schema for the resource.
func (r *spannerTableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				Description: "The name of the table.\n" +
					"The name must satisfy the expression `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						regexp.MustCompile(utils.SpannerGoogleSqlTableIdRegex),
						regexp.MustCompile(utils.SpannerPostgresSqlTableIdRegex),
					}, "Name must be a valid Spanner Table ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
			},
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID in which the table belongs.",
			},
			"instance": schema.StringAttribute{
				Required:    true,
				Description: "The name of the Spanner instance.",
			},
			"database": schema.StringAttribute{
				Required:    true,
				Description: "The name of the parent database.",
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
									Description: "The name of the column.\n" +
										"The name must contain only letters (a-z, A-Z), numbers (0-9), or underscores (_), and must start with a letter and not end in an underscore.\n" +
										"The maximum length is 128 characters.",
									Validators: []validator.String{
										validators.RegexMatches([]*regexp.Regexp{
											regexp.MustCompile(utils.SpannerGoogleSqlColumnIdRegex),
											regexp.MustCompile(utils.SpannerPostgresSqlColumnIdRegex),
										}, "Name must be a valid Spanner Column ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
									},
								},
								"is_primary_key": schema.BoolAttribute{
									Optional: true,
									Description: "Indicates if the column is part of the primary key.\n" +
										"Multiple columns can be specified as primary keys to create a composite primary key.\n" +
										"Primary key columns must be non-null.",
								},
								"auto_increment": schema.BoolAttribute{
									Optional: true,
									Description: "Indicates if the column is auto-incrementing.\n" +
										"The column must be of type `INT64` or `FLOAT64`.",
								},
								"unique": schema.BoolAttribute{
									Optional:    true,
									Description: "Indicates if the column is unique.",
								},
								"type": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf(services.SpannerTableDataTypes...),
									},
									Description: "The data type of the column.\n" +
										"Valid types are: `BOOL`, `INT64`, `FLOAT64`, `STRING`, `BYTES`, `DATE`, `TIMESTAMP`, `JSON`.",
								},
								"size": schema.Int64Attribute{
									Optional:    true,
									Description: "The maximum size of the column.",
								},
								"precision": schema.Int64Attribute{
									Optional: true,
									Description: "The maximum number of digits in the column.\n" +
										"This is only applicable to columns of type `FLOAT64`.\n" +
										"The maximum is 17",
								},
								"scale": schema.Int64Attribute{
									Optional: true,
									Description: "The maximum number of digits after the decimal point in the column.\n" +
										"This is only applicable to columns of type `FLOAT64`.",
								},
								"required": schema.BoolAttribute{
									Optional:    true,
									Description: "Indicates if the column is required.",
								},
								"default_value": schema.StringAttribute{
									Optional: true,
									Description: "The default value of the column.\n" +
										"The default value must be compatible with the column type.\n" +
										"For example, a default value of \"true\" is valid for a `BOOL` or `STRING` column, but not for an `INT64` column.",
								},
							},
						},
						Description: "The columns of the table.",
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
									Description: "The name of the index.\n" +
										"The name must contain only letters (a-z, A-Z), numbers (0-9), or hyphens (-), and must start with a letter and not end in a hyphen.",
									Validators: []validator.String{
										validators.RegexMatches([]*regexp.Regexp{
											regexp.MustCompile(utils.SpannerGoogleSqlIndexIdRegex),
											regexp.MustCompile(utils.SpannerPostgresSqlIndexIdRegex),
										}, "Name must be a valid Spanner Index ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
									},
								},
								"columns": schema.ListNestedAttribute{
									Required: true,
									CustomType: types.ListType{
										ElemType: types.ObjectType{
											AttrTypes: spannerTableIndexColumn{}.attrTypes(),
										},
									},
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"name": schema.StringAttribute{
												Required:    true,
												Description: "The name of the column that makes up the index.",
												Validators: []validator.String{
													validators.RegexMatches([]*regexp.Regexp{
														regexp.MustCompile(utils.SpannerGoogleSqlColumnIdRegex),
														regexp.MustCompile(utils.SpannerPostgresSqlColumnIdRegex),
													}, "Name must be a valid Spanner Column ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
												},
											},
											"order": schema.StringAttribute{
												Optional: true,
												Description: "The sorting order of the column in the index.\n" +
													"Valid values are: `asc` or `desc`. If not specified the default is `asc`.",
												Validators: []validator.String{
													stringvalidator.OneOf(services.SpannerTableIndexColumnOrders...),
												},
											},
										},
									},
									Description: "The columns that make up the index.\n" +
										"The order of the columns is significant.",
								},
								"unique": schema.BoolAttribute{
									Optional:    true,
									Description: "Indicates if the index is unique.",
								},
							},
						},
						Description: "The indices/indexes of the table.",
					},
				},
				Description: "The schema of the table.",
			},
		},
		Description: "A Google Cloud Spanner table resource.\n" +
			"This resource manages the schema of a table in a Google Cloud Spanner database.",
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
	table := &services.SpannerTable{
		Name: "",
		Schema: &services.SpannerTableSchema{
			Columns: nil,
			Indices: nil,
		},
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	databaseId := plan.Database.ValueString()
	tableId := plan.Name.ValueString()

	// Populate schema if any
	if plan.Schema != nil {
		tableSchema := &services.SpannerTableSchema{
			Columns: nil,
			Indices: nil,
		}

		if !plan.Schema.Columns.IsNull() {
			columns := make([]spannerTableColumn, 0, len(plan.Schema.Columns.Elements()))
			d := plan.Schema.Columns.ElementsAs(ctx, &columns, false)
			if d.HasError() {
				tflog.Error(ctx, fmt.Sprintf("Error reading columns: %v", d))
				return
			}
			diags.Append(d...)

			for _, column := range columns {
				col := &services.SpannerTableColumn{}

				// Populate column name
				if !column.Name.IsNull() {
					col.Name = column.Name.ValueString()
				}

				// Populate is primary key
				if !column.IsPrimaryKey.IsNull() {
					col.IsPrimaryKey = wrapperspb.Bool(column.IsPrimaryKey.ValueBool())
				}

				// Populate auto increment
				if !column.AutoIncrement.IsNull() {
					col.AutoIncrement = wrapperspb.Bool(column.AutoIncrement.ValueBool())
				}

				// Populate unique
				if !column.Unique.IsNull() {
					col.Unique = wrapperspb.Bool(column.Unique.ValueBool())
				}

				// Populate type
				if !column.Type.IsNull() {
					col.Type = column.Type.ValueString()
				}

				// Populate size
				if !column.Size.IsNull() {
					col.Size = wrapperspb.Int64(column.Size.ValueInt64())
				}

				// Populate precision
				if !column.Precision.IsNull() {
					col.Precision = wrapperspb.Int64(column.Precision.ValueInt64())
				}

				// Populate scale
				if !column.Scale.IsNull() {
					col.Scale = wrapperspb.Int64(column.Scale.ValueInt64())
				}

				// Populate required
				if !column.Required.IsNull() {
					col.Required = wrapperspb.Bool(column.Required.ValueBool())
				}

				// Populate default value
				if !column.DefaultValue.IsNull() {
					col.DefaultValue = wrapperspb.String(column.DefaultValue.ValueString())
				}

				tableSchema.Columns = append(tableSchema.Columns, col)
			}
		}

		if !plan.Schema.Indices.IsNull() {
			indices := make([]spannerTableIndex, 0, len(plan.Schema.Indices.Elements()))
			d := plan.Schema.Indices.ElementsAs(ctx, &indices, false)
			if d.HasError() {
				tflog.Error(ctx, fmt.Sprintf("Error reading indices: %v", d))
				return
			}
			diags.Append(d...)

			for _, index := range indices {
				idx := &services.SpannerTableIndex{}

				// Populate index name
				if !index.Name.IsNull() {
					idx.Name = index.Name.ValueString()
				}

				// Populate unique
				if !index.Unique.IsNull() {
					idx.Unique = wrapperspb.Bool(index.Unique.ValueBool())
				}

				// Populate columns
				if !index.Columns.IsNull() {
					columns := make([]spannerTableIndexColumn, 0, len(index.Columns.Elements()))
					d := index.Columns.ElementsAs(ctx, &columns, false)
					if d.HasError() {
						tflog.Error(ctx, fmt.Sprintf("Error reading index columns: %v", d))
						return
					}
					diags.Append(d...)

					for _, column := range columns {
						order := services.SpannerTableIndexColumnOrder_ASC
						switch column.Order.ValueString() {
						case "asc":
							order = services.SpannerTableIndexColumnOrder_ASC
						case "desc":
							order = services.SpannerTableIndexColumnOrder_DESC
						}
						idx.Columns = append(idx.Columns, &services.SpannerTableIndexColumn{
							Name:  column.Name.ValueString(),
							Order: order,
						})
					}
				}

				tableSchema.Indices = append(tableSchema.Indices, idx)
			}
		}

		table.Schema = tableSchema
	}

	// Create table
	_, err := r.config.SpannerService.CreateSpannerTable(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
		tableId,
		table,
	)
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
	instanceName := state.Instance.ValueString()
	databaseId := state.Database.ValueString()
	tableId := state.Name.ValueString()

	// Get table from API
	table, err := r.config.SpannerService.GetSpannerTable(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
	)
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
	if table.Schema != nil {
		s := &spannerTableSchema{}
		if table.Schema.Columns != nil {
			columns := make([]*spannerTableColumn, 0)
			for _, column := range table.Schema.Columns {
				col := &spannerTableColumn{
					Name: types.StringValue(column.Name),
				}

				// Get primary key
				if column.IsPrimaryKey != nil {
					col.IsPrimaryKey = types.BoolValue(column.IsPrimaryKey.GetValue())
				}

				// Get auto increment
				if column.AutoIncrement != nil {
					col.AutoIncrement = types.BoolValue(column.AutoIncrement.GetValue())
				}

				// Get unique
				if column.Unique != nil {
					col.Unique = types.BoolValue(column.Unique.GetValue())
				}

				// Get type
				if column.Type != "" {
					col.Type = types.StringValue(column.Type)
				}

				// Get size
				if column.Size != nil {
					col.Size = types.Int64Value(column.Size.GetValue())
				}

				// Get precision
				if column.Precision != nil {
					col.Precision = types.Int64Value(column.Precision.GetValue())
				}

				// Get scale
				if column.Scale != nil {
					col.Scale = types.Int64Value(column.Scale.GetValue())
				}

				// Get required
				if column.Required != nil {
					col.Required = types.BoolValue(column.Required.GetValue())
				}

				// Get default value
				if column.DefaultValue != nil {
					col.DefaultValue = types.StringValue(column.DefaultValue.GetValue())
				}

				columns = append(columns, col)
			}

			generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
				AttrTypes: spannerTableColumn{}.attrTypes(),
			}, columns)
			diags.Append(d...)

			s.Columns = generatedList
		}
		if table.Schema.Indices != nil {
			indices := make([]*spannerTableIndex, 0, len(table.Schema.Indices))
			for _, index := range table.Schema.Indices {
				idx := &spannerTableIndex{
					Name:    types.StringValue(index.Name),
					Columns: types.List{},
				}

				// Get unique
				if index.Unique != nil {
					idx.Unique = types.BoolValue(index.Unique.GetValue())
				}

				// Get columns
				columns := make([]*spannerTableIndexColumn, 0)
				for _, column := range index.Columns {
					columns = append(columns, &spannerTableIndexColumn{
						Name:  types.StringValue(column.Name),
						Order: types.StringValue(column.Order.String()),
					})
				}
				generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
					AttrTypes: spannerTableIndexColumn{}.attrTypes(),
				}, columns)
				diags.Append(d...)

				idx.Columns = generatedList

				indices = append(indices, idx)
			}

			generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
				AttrTypes: spannerTableIndex{}.attrTypes(),
			}, indices)
			diags.Append(d...)

			s.Indices = generatedList
		}

		state.Schema = s
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

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	databaseId := plan.Database.ValueString()
	tableId := plan.Name.ValueString()

	// Generate table from plan
	table := &services.SpannerTable{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
		Schema: &services.SpannerTableSchema{
			Columns: nil,
			Indices: nil,
		},
	}

	// Populate schema if any
	if plan.Schema != nil {
		tableSchema := &services.SpannerTableSchema{
			Columns: nil,
			Indices: nil,
		}

		if !plan.Schema.Columns.IsNull() {
			columns := make([]spannerTableColumn, 0, len(plan.Schema.Columns.Elements()))
			d := plan.Schema.Columns.ElementsAs(ctx, &columns, false)
			if d.HasError() {
				tflog.Error(ctx, fmt.Sprintf("Error reading columns: %v", d))
				return
			}
			diags.Append(d...)

			for _, column := range columns {
				col := &services.SpannerTableColumn{}

				// Populate column name
				if !column.Name.IsNull() {
					col.Name = column.Name.ValueString()
				}

				// Populate is primary key
				if !column.IsPrimaryKey.IsNull() {
					col.IsPrimaryKey = wrapperspb.Bool(column.IsPrimaryKey.ValueBool())
				}

				// Populate auto increment
				if !column.AutoIncrement.IsNull() {
					col.AutoIncrement = wrapperspb.Bool(column.AutoIncrement.ValueBool())
				}

				// Populate unique
				if !column.Unique.IsNull() {
					col.Unique = wrapperspb.Bool(column.Unique.ValueBool())
				}

				// Populate type
				if !column.Type.IsNull() {
					col.Type = column.Type.ValueString()
				}

				// Populate size
				if !column.Size.IsNull() {
					col.Size = wrapperspb.Int64(column.Size.ValueInt64())
				}

				// Populate precision
				if !column.Precision.IsNull() {
					col.Precision = wrapperspb.Int64(column.Precision.ValueInt64())
				}

				// Populate scale
				if !column.Scale.IsNull() {
					col.Scale = wrapperspb.Int64(column.Scale.ValueInt64())
				}

				// Populate required
				if !column.Required.IsNull() {
					col.Required = wrapperspb.Bool(column.Required.ValueBool())
				}

				// Populate default value
				if !column.DefaultValue.IsNull() {
					col.DefaultValue = wrapperspb.String(column.DefaultValue.ValueString())
				}

				tableSchema.Columns = append(tableSchema.Columns, col)
			}
		}

		if !plan.Schema.Indices.IsNull() {
			indices := make([]spannerTableIndex, 0, len(plan.Schema.Indices.Elements()))
			d := plan.Schema.Indices.ElementsAs(ctx, &indices, false)
			if d.HasError() {
				tflog.Error(ctx, fmt.Sprintf("Error reading indices: %v", d))
				return
			}
			diags.Append(d...)

			for _, index := range indices {
				idx := &services.SpannerTableIndex{}

				// Populate index name
				if !index.Name.IsNull() {
					idx.Name = index.Name.ValueString()
				}

				// Populate unique
				if !index.Unique.IsNull() {
					idx.Unique = wrapperspb.Bool(index.Unique.ValueBool())
				}

				// Populate columns
				if !index.Columns.IsNull() {
					columns := make([]spannerTableIndexColumn, 0, len(index.Columns.Elements()))
					d := index.Columns.ElementsAs(ctx, &columns, false)
					if d.HasError() {
						tflog.Error(ctx, fmt.Sprintf("Error reading index columns: %v", d))
						return
					}
					diags.Append(d...)

					for _, column := range columns {
						order := services.SpannerTableIndexColumnOrder_ASC
						switch column.Order.ValueString() {
						case "asc":
							order = services.SpannerTableIndexColumnOrder_ASC
						case "desc":
							order = services.SpannerTableIndexColumnOrder_DESC
						}
						idx.Columns = append(idx.Columns, &services.SpannerTableIndexColumn{
							Name:  column.Name.ValueString(),
							Order: order,
						})
					}
				}

				tableSchema.Indices = append(tableSchema.Indices, idx)
			}
		}

		table.Schema = tableSchema
	}

	// Update table
	_, err := r.config.SpannerService.UpdateSpannerTable(ctx, table, &fieldmaskpb.FieldMask{
		Paths: []string{"schema.columns", "schema.indices"},
	}, false)
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
	instanceName := state.Instance.ValueString()
	databaseId := state.Database.ValueString()
	tableId := state.Name.ValueString()

	// Delete existing database
	_, err := r.config.SpannerService.DeleteSpannerTable(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId))
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), databaseName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerTableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *spannerTableResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
