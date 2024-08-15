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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	Name           types.String        `tfsdk:"name"`
	Project        types.String        `tfsdk:"project"`
	Instance       types.String        `tfsdk:"instance"`
	Database       types.String        `tfsdk:"database"`
	Schema         *spannerTableSchema `tfsdk:"schema"`
	PreventDestroy types.Bool          `tfsdk:"prevent_destroy"`
}

type spannerTableSchema struct {
	Columns types.List `tfsdk:"columns"`
}

type spannerTableColumn struct {
	Name           types.String `tfsdk:"name"`
	IsPrimaryKey   types.Bool   `tfsdk:"is_primary_key"`
	IsComputed     types.Bool   `tfsdk:"is_computed"`
	ComputationDdl types.String `tfsdk:"computation_ddl"`
	AutoIncrement  types.Bool   `tfsdk:"auto_increment"`
	Unique         types.Bool   `tfsdk:"unique"`
	Type           types.String `tfsdk:"type"`
	Size           types.Int64  `tfsdk:"size"`
	Precision      types.Int64  `tfsdk:"precision"`
	Scale          types.Int64  `tfsdk:"scale"`
	Required       types.Bool   `tfsdk:"required"`
	DefaultValue   types.String `tfsdk:"default_value"`
	ProtoPackage   types.String `tfsdk:"proto_package"`
	FileDescriptor types.String `tfsdk:"file_descriptor"`
}

func (o spannerTableColumn) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":            types.StringType,
		"is_primary_key":  types.BoolType,
		"is_computed":     types.BoolType,
		"computation_ddl": types.StringType,
		"auto_increment":  types.BoolType,
		"unique":          types.BoolType,
		"type":            types.StringType,
		"size":            types.Int64Type,
		"precision":       types.Int64Type,
		"scale":           types.Int64Type,
		"required":        types.BoolType,
		"default_value":   types.StringType,
		"proto_package":   types.StringType,
		"file_descriptor": types.StringType,
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID in which the table belongs.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance": schema.StringAttribute{
				Required:    true,
				Description: "The name of the Spanner instance.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Required:    true,
				Description: "The name of the parent database.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
										"Primary key columns must be non-null.\n" +
										"**Changing this value will cause a table replace**.",
								},
								"is_computed": schema.BoolAttribute{
									Optional: true,
									Description: "Indicates if the column is a computed column.\n" +
										"Computed columns are generated values based on other columns in the table.\n" +
										"A common use case is to generate a column from a PROTO column field.\n" +
										"This should be accompanied by a `computation_ddl` field.\n" +
										"**Changing this value will cause a table replace**.",
								},
								"computation_ddl": schema.StringAttribute{
									Optional: true,
									Description: "The DDL expression for the computed column.\n" +
										"This is only applicable to columns where `is_computed` is true.\n" +
										"The expression must be a valid SQL expression that generates a value for the column.\n" +
										"Example: `column1 + column2`, or `proto_column.field`.\n" +
										"**Changing this value will cause a table replace**.",
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
										"Valid types are: `BOOL`, `INT64`, `FLOAT64`, `STRING`, `BYTES`, `DATE`, `TIMESTAMP`, `JSON`, `PROTO`, `ARRAY<STRING>`, `ARRAY<INT64>`, `ARRAY<FLOAT32>`, `ARRAY<FLOAT64>`.\n" +
										"**Changing this value will cause a table replace**.",
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
								"proto_package": schema.StringAttribute{
									Optional: true,
									Description: "The full name of the proto message to be used in the column.\n" +
										"The name must be a valid package name including the message name.\n" +
										"This field is only required for columns of type `PROTO`\n" +
										"Example: \"com.example.Message\", where `com.example` is the package name and `Message` is the message name.",
								},
								"file_descriptor": schema.StringAttribute{
									Optional: true,
									Description: "The url/path to the file descriptor set of the column.\n" +
										"The file descriptor set must be a valid file descriptor set containing the specified `proto_package`.\n" +
										"The path must point to a valid `.pb` file.\n" +
										"You can generate one using the `protoc` compiler. See https://cloud.google.com/spanner/docs/reference/standard-sql/protocol-buffers#create_a_protocol_buffer.\n" +
										"This field is only compatible for columns of type `PROTO`.\n" +
										"**This field is not required if the database is already populated with the necessary proto bundles.**\n" +
										"One of the following prefixes must be used to indicate the location of the file descriptor set:\n" +
										"	- **gcs:** - Indicates the file is stored in a Google Cloud Storage Bucket.\n" +
										"	Example: \"gcs:gs://path/to/your/file.pb\".\n" +
										"	- **url:** - **Experimental**. Indicates the file is stored on a remote server accessible via HTTPS.\n" +
										"	Example: \"url:https://path/to/your/file.pb\".",
								},
							},
						},
						Description: "The columns of the table.",
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
								// Create a map of the columns by name
								type PriorAndCurrentColumns struct {
									Prior   *spannerTableColumn
									Current *spannerTableColumn
								}
								columnsMap := make(map[string]*PriorAndCurrentColumns)

								// Get the columns prior to the plan
								priorColumns := make([]spannerTableColumn, 0, len(req.StateValue.Elements()))
								d := req.StateValue.ElementsAs(ctx, &priorColumns, false)
								if d.HasError() {
									resp.Diagnostics.Append(d...)
									return
								}
								for _, column := range priorColumns {
									if _, ok := columnsMap[column.Name.ValueString()]; !ok {
										columnsMap[column.Name.ValueString()] = &PriorAndCurrentColumns{}
									}
									columnsMap[column.Name.ValueString()].Prior = &column
								}

								// Get the columns after the plan
								currentColumns := make([]spannerTableColumn, 0, len(req.PlanValue.Elements()))
								d = req.PlanValue.ElementsAs(ctx, &currentColumns, false)
								if d.HasError() {
									resp.Diagnostics.Append(d...)
									return
								}
								for _, column := range currentColumns {
									if _, ok := columnsMap[column.Name.ValueString()]; !ok {
										columnsMap[column.Name.ValueString()] = &PriorAndCurrentColumns{}
									}
									columnsMap[column.Name.ValueString()].Current = &column
								}

								// Check if the columns are the same.
								// Columns that are new do not require a replace, unless a primary key is added.
								// Columns that are removed do not require a replace, unless they are part of the primary key.
								// Columns that are updated require a replace if: the column type is changed,
								// the primary key status is changed, or the column's computation_ddl is changed.
								for name, columns := range columnsMap {
									// Column is new
									if columns.Prior == nil && columns.Current != nil {
										// Check if the column is a primary key
										if !columns.Current.IsPrimaryKey.IsNull() && columns.Current.IsPrimaryKey.ValueBool() {
											resp.RequiresReplace = true
											resp.Diagnostics.AddWarning(fmt.Sprintf("Column %q requires a table replace", name), fmt.Sprintf("Column %q is a new primary key column and requires a table replace", name))
										}
										continue
									}

									// Column is removed
									if columns.Current == nil && columns.Prior != nil {
										// Check if the column is a primary key
										if !columns.Prior.IsPrimaryKey.IsNull() && columns.Prior.IsPrimaryKey.ValueBool() {
											resp.RequiresReplace = true
											resp.Diagnostics.AddWarning(fmt.Sprintf("Column %q requires a table replace", name), fmt.Sprintf("Column %q is a removed primary key column and requires a table replace", name))
										}
										continue
									}

									// Column type is changed
									// Type is required, so we can safely assume it is not null
									if columns.Prior.Type.ValueString() != columns.Current.Type.ValueString() {
										resp.RequiresReplace = true
										resp.Diagnostics.AddWarning(fmt.Sprintf("Column %q requires a table replace", name), fmt.Sprintf("Column %q has a changed type and requires a table replace", name))
									}

									// Column primary key status is changed
									// This is not required, so we also need to check if it is null
									if (!columns.Prior.IsPrimaryKey.IsNull() && !columns.Current.IsPrimaryKey.IsNull() && columns.Prior.IsPrimaryKey.ValueBool() != columns.Current.IsPrimaryKey.ValueBool()) ||
										(columns.Prior.IsPrimaryKey.IsNull() && !columns.Current.IsPrimaryKey.IsNull() && columns.Current.IsPrimaryKey.ValueBool()) ||
										(!columns.Prior.IsPrimaryKey.IsNull() && columns.Prior.IsPrimaryKey.ValueBool() && columns.Current.IsPrimaryKey.IsNull()) {
										resp.RequiresReplace = true
										resp.Diagnostics.AddWarning(fmt.Sprintf("Column %q requires a table replace", name), fmt.Sprintf("Column %q has a changed primary key status and requires a table replace", name))
									}

									// Column is computed and computation_ddl is changed
									// Both fields are required but only if at least one is set
									if (!columns.Prior.IsComputed.IsNull() && columns.Prior.IsComputed.ValueBool() && !columns.Current.IsComputed.IsNull() && columns.Current.IsComputed.ValueBool() &&
										columns.Prior.ComputationDdl.ValueString() != columns.Current.ComputationDdl.ValueString()) ||
										(!columns.Prior.IsComputed.IsNull() && columns.Prior.IsComputed.ValueBool() && (columns.Current.IsComputed.IsNull() || !columns.Current.IsComputed.ValueBool())) {
										resp.RequiresReplace = true
										resp.Diagnostics.AddWarning(fmt.Sprintf("Column %q requires a table replace", name), fmt.Sprintf("Column %q has a changed computation_ddl or is_computed has been disabled and requires a table replace", name))
									}
								}

							},
								"If certain values of any of the columns change, Terraform will destroy and recreate the table.", "If certain values of any of the columns change, Terraform will destroy and recreate the table."),
						},
					},
				},
				Description: "The schema of the table.",
			},
			"prevent_destroy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Prevent the table from being destroyed.\n" +
					"**This only applies to the terraform state and does not prevent the actual table from being deleted via another source.**",
				Default: booldefault.StaticBool(true),
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

				// Populate is computed
				if !column.IsComputed.IsNull() {
					col.IsComputed = wrapperspb.Bool(column.IsComputed.ValueBool())
				}

				// Populate computation ddl
				if !column.ComputationDdl.IsNull() {
					col.ComputationDdl = wrapperspb.String(column.ComputationDdl.ValueString())
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

				// Populate ProtoFileDescriptorSet
				if !column.ProtoPackage.IsNull() || !column.FileDescriptor.IsNull() {
					col.ProtoFileDescriptorSet = &services.ProtoFileDescriptorSet{}

					// Populate proto package
					if !column.ProtoPackage.IsNull() {
						col.ProtoFileDescriptorSet.ProtoPackage = wrapperspb.String(column.ProtoPackage.ValueString())
					}

					// Populate file descriptor
					if !column.FileDescriptor.IsNull() {
						col.ProtoFileDescriptorSet.FileDescriptorSetPath = wrapperspb.String(column.FileDescriptor.ValueString())

						if strings.HasPrefix(column.FileDescriptor.ValueString(), "gcs:") {
							col.ProtoFileDescriptorSet.FileDescriptorSetPathSource = services.ProtoFileDescriptorSetSourceGcs
						}

						if strings.HasPrefix(column.FileDescriptor.ValueString(), "url:") {
							col.ProtoFileDescriptorSet.FileDescriptorSetPathSource = services.ProtoFileDescriptorSetSourceUrl
						}
					}
				}

				tableSchema.Columns = append(tableSchema.Columns, col)
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
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

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

				// Get computed
				if column.IsComputed != nil {
					col.IsComputed = types.BoolValue(column.IsComputed.GetValue())
				}

				// Get computation ddl
				if column.ComputationDdl != nil {
					col.ComputationDdl = types.StringValue(column.ComputationDdl.GetValue())
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

				if column.ProtoFileDescriptorSet != nil {
					// Get proto package
					if column.ProtoFileDescriptorSet.ProtoPackage != nil {
						col.ProtoPackage = types.StringValue(column.ProtoFileDescriptorSet.ProtoPackage.GetValue())
					}

					// Get file descriptor set path
					if column.ProtoFileDescriptorSet.FileDescriptorSetPath != nil {
						col.FileDescriptor = types.StringValue(column.ProtoFileDescriptorSet.FileDescriptorSetPath.GetValue())
					}
				}

				columns = append(columns, col)
			}

			generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
				AttrTypes: spannerTableColumn{}.attrTypes(),
			}, columns)
			diags.Append(d...)

			s.Columns = generatedList
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
		},
	}

	// Populate schema if any
	if plan.Schema != nil {
		tableSchema := &services.SpannerTableSchema{
			Columns: nil,
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

				// Populate is computed
				if !column.IsComputed.IsNull() {
					col.IsComputed = wrapperspb.Bool(column.IsComputed.ValueBool())
				}

				// Populate computation ddl
				if !column.ComputationDdl.IsNull() {
					col.ComputationDdl = wrapperspb.String(column.ComputationDdl.ValueString())
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

				// Populate ProtoFileDescriptorSet
				if !column.ProtoPackage.IsNull() || !column.FileDescriptor.IsNull() {
					col.ProtoFileDescriptorSet = &services.ProtoFileDescriptorSet{}

					// Populate proto package
					if !column.ProtoPackage.IsNull() {
						col.ProtoFileDescriptorSet.ProtoPackage = wrapperspb.String(column.ProtoPackage.ValueString())
					}

					// Populate file descriptor
					if !column.FileDescriptor.IsNull() {
						col.ProtoFileDescriptorSet.FileDescriptorSetPath = wrapperspb.String(column.FileDescriptor.ValueString())

						if strings.HasPrefix(column.FileDescriptor.ValueString(), "gcs:") {
							col.ProtoFileDescriptorSet.FileDescriptorSetPathSource = services.ProtoFileDescriptorSetSourceGcs
						}

						if strings.HasPrefix(column.FileDescriptor.ValueString(), "url:") {
							col.ProtoFileDescriptorSet.FileDescriptorSetPathSource = services.ProtoFileDescriptorSetSourceUrl
						}
					}
				}

				tableSchema.Columns = append(tableSchema.Columns, col)
			}
		}

		table.Schema = tableSchema
	}

	// Update table
	_, err := r.config.SpannerService.UpdateSpannerTable(ctx, table, &fieldmaskpb.FieldMask{
		Paths: []string{"schema.columns"},
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

	// Check if prevent_destroy is set to true
	if state.PreventDestroy.ValueBool() {
		resp.Diagnostics.AddError(
			"Error Deleting Table",
			"Table ("+state.Name.ValueString()+") is protected from deletion by terraform configuration. Set `prevent_destroy` to false.",
		)
		return
	}

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
	// projects/{project}/instances/{instance}/databases/{database}/tables/{tables}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 8 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]
	tableName := importIDParts[7]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), tableName)...)
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

func (r *spannerTableResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data spannerTableModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Check if schema is provided
	if data.Schema == nil {
		resp.Diagnostics.AddAttributeWarning(
			path.Root("schema"),
			"Missing Schema Configuration",
			"Expected schema to be configured with columns. "+
				"The resource may return unexpected results.",
		)
		return
	}

	// Check if at least one column is provided
	if data.Schema.Columns.IsNull() || len(data.Schema.Columns.Elements()) == 0 {
		resp.Diagnostics.AddAttributeWarning(
			path.Root("schema.columns"),
			"Missing Column Configuration",
			"Expected at least one column to be configured. "+
				"The resource may return unexpected results.",
		)
		return
	}

	columns := make([]spannerTableColumn, 0, len(data.Schema.Columns.Elements()))
	d := data.Schema.Columns.ElementsAs(ctx, &columns, false)
	resp.Diagnostics.Append(d...)
	if d.HasError() {
		return
	}

	for i, column := range columns {
		// If column type is PROTO, check if proto_package and file_descriptor are provided
		if column.Type.ValueString() == "PROTO" {
			if column.ProtoPackage.IsNull() {
				resp.Diagnostics.AddAttributeWarning(
					path.Root("schema.columns").AtListIndex(i).AtName("proto_package"),
					"Missing Column Configuration",
					"Expected proto_package to be configured for columns of type PROTO. "+
						"The resource may return unexpected results.",
				)
			}

			// TODO: Uncomment when file_descriptor is required
			//if column.FileDescriptor.IsNull() {
			//	resp.Diagnostics.AddAttributeWarning(
			//		path.Root("schema.columns").AtListIndex(i).AtName("file_descriptor"),
			//		"Missing Column Configuration",
			//		"Expected file_descriptor to be configured for columns of type PROTO. "+
			//			"The resource may return unexpected results.",
			//	)
			//}
		}

		// If column is computed, check if computation_ddl is provided
		if !column.IsComputed.IsNull() && column.IsComputed.ValueBool() {
			if column.ComputationDdl.IsNull() || column.ComputationDdl.ValueString() == "" {
				resp.Diagnostics.AddAttributeWarning(
					path.Root("schema.columns").AtListIndex(i).AtName("computation_ddl"),
					"Missing Column Configuration",
					"Expected computation_ddl to be configured for computed columns. "+
						"The resource may return unexpected results.",
				)
			}
		}
	}

	return
}

func (r *spannerTableResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		//resourcevalidator.RequiredTogether(
		//	path.MatchRoot("schema.columns").AtAnyListIndex().AtName("type").AtSetValue(types.StringValue("PROTO")),
		//	path.MatchRoot("schema.columns").AtAnyListIndex().AtName("proto_package"),
		//	path.MatchRoot("schema.columns").AtAnyListIndex().AtName("file_descriptor"),
		//),

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
