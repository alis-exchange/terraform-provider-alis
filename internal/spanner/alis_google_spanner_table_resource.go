package spanner

import (
	"context"
	"fmt"
	"regexp"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/names"
	tableschema "terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
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

// spannerTableResource manages the schema of a Spanner table
// (alis_google_spanner_table). Only schema.columns can change in place;
// identifying attributes carry RequiresReplace plan modifiers, and column
// changes that DDL cannot apply in place force a replace via
// tableColumnsRequireReplace.
type spannerTableResource struct {
	config *internal.ProviderConfig
}

type spannerTableModel struct {
	Name           types.String            `tfsdk:"name"`
	Project        types.String            `tfsdk:"project"`
	Instance       types.String            `tfsdk:"instance"`
	Database       types.String            `tfsdk:"database"`
	Schema         *spannerTableSchema     `tfsdk:"schema"`
	Interleave     *spannerTableInterleave `tfsdk:"interleave"`
	PreventDestroy types.Bool              `tfsdk:"prevent_destroy"`
}

type spannerTableSchema struct {
	Columns types.List `tfsdk:"columns"`
}

// spannerTableColumn is the Terraform model of one table column. Null
// attributes mean "not configured"; tableColumnsToSchema maps them to nil
// wrappers so the schema layer can tell unset apart from an explicit
// false/zero, and tableColumnsToModel maps nil back to null.
type spannerTableColumn struct {
	Name           types.String `tfsdk:"name"`
	IsPrimaryKey   types.Bool   `tfsdk:"is_primary_key"`
	IsComputed     types.Bool   `tfsdk:"is_computed"`
	ComputationDdl types.String `tfsdk:"computation_ddl"`
	IsStored       types.Bool   `tfsdk:"is_stored"`
	AutoUpdateTime types.Bool   `tfsdk:"auto_update_time"`
	Type           types.String `tfsdk:"type"`
	Size           types.Int64  `tfsdk:"size"`
	Required       types.Bool   `tfsdk:"required"`
	DefaultValue   types.String `tfsdk:"default_value"`
	ProtoPackage   types.String `tfsdk:"proto_package"`
}

// attrTypes returns the attribute types of the column object, used to build
// the typed columns list.
func (o spannerTableColumn) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":             types.StringType,
		"is_primary_key":   types.BoolType,
		"is_computed":      types.BoolType,
		"computation_ddl":  types.StringType,
		"is_stored":        types.BoolType,
		"auto_update_time": types.BoolType,
		"type":             types.StringType,
		"size":             types.Int64Type,
		"required":         types.BoolType,
		"default_value":    types.StringType,
		"proto_package":    types.StringType,
	}
}

type spannerTableInterleave struct {
	ParentTable types.String `tfsdk:"parent_table"`
	OnDelete    types.String `tfsdk:"on_delete"`
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
								"is_stored": schema.BoolAttribute{
									Optional: true,
									Description: "Indicates if the generated column is stored.\n" +
										"This is only applicable to columns where `is_computed` is true.\n" +
										"Stored columns are physically stored in the table and can be indexed.\n" +
										"Non-stored columns are not physically stored in the table and are computed on the fly.\n" +
										"**Changing this value will cause a table replace**.",
								},
								"auto_update_time": schema.BoolAttribute{
									Optional: true,
									Description: "Indicates if the column auto populates on row update.\n" +
										"The column must be of type `TIMESTAMP`.",
								},
								"type": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf(tableschema.SpannerTableDataTypes...),
									},
									Description: "The data type of the column.\n" +
										"Valid types are: `BOOL`, `INT64`, `FLOAT64`, `STRING`, `BYTES`, `DATE`, `TIMESTAMP`, `JSON`, `PROTO`, `ARRAY<STRING>`, `ARRAY<INT64>`, `ARRAY<FLOAT32>`, `ARRAY<FLOAT64>`.\n" +
										"**Changing this value will cause a table replace**.",
								},
								"size": schema.Int64Attribute{
									Optional:    true,
									Description: "The maximum size of the column.",
								},
								"required": schema.BoolAttribute{
									Optional:    true,
									Description: "Indicates if the column is required.",
								},
								"default_value": schema.StringAttribute{
									Optional: true,
									Description: "Expression used as the column default in Spanner `DEFAULT (...)`.\n" +
										"It must be valid for the column type: literals (e.g. `10.0` for `FLOAT64`, `\"true\"` for `BOOL` or `STRING`) or Spanner default expressions.\n" +
										"Examples of expressions: `GENERATE_UUID()` for a `STRING` (or `BYTES`) primary key; `GET_NEXT_SEQUENCE_VALUE(SEQUENCE my_sequence)` for an `INT64` column when `my_sequence` exists in the same database.\n" +
										"Do not wrap the value in an extra pair of parentheses; the provider emits `DEFAULT (<this value>)`.",
								},
								"proto_package": schema.StringAttribute{
									Optional: true,
									Description: "The full name of the proto message to be used in the column.\n" +
										"The name must be a valid package name including the message name.\n" +
										"This field is only required for columns of type `PROTO`\n" +
										"Example: \"com.example.Message\", where `com.example` is the package name and `Message` is the message name.",
								},
							},
						},
						Description: "The columns of the table.",
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplaceIf(tableColumnsRequireReplace,
								"If certain values of any of the columns change, Terraform will destroy and recreate the table.", "If certain values of any of the columns change, Terraform will destroy and recreate the table."),
						},
					},
				},
				Description: "The schema of the table.",
			},
			"interleave": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"parent_table": schema.StringAttribute{
						Required: true,
						Description: "The name of the parent table to interleave in.\n" +
							"The parent table must be in the same database.\n" +
							"**Changing this value will cause a table replace**.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"on_delete": schema.StringAttribute{
						Optional: true,
						Description: "The action to take on delete.\n" +
							"Supported values are `CASCADE`, `NO_ACTION`.\n" +
							"Setting this value to `CASCADE` signifies that when a row from the parent table is deleted, its child rows are automatically deleted as well.\n" +
							"The default value is `NO_ACTION`.\n" +
							"**Changing this value will cause a table replace**.",
						Validators: []validator.String{
							stringvalidator.OneOf(tableschema.SpannerTableConstraintActions...),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
				Description: "The interleave configuration of the table.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
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
	table := &tableschema.SpannerTable{
		Name: "",
		Schema: &tableschema.SpannerTableSchema{
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
		columns, d := tableColumnsToSchema(ctx, plan.Schema.Columns)
		if d.HasError() {
			tflog.Error(ctx, fmt.Sprintf("Error reading columns: %v", d))
			return
		}
		table.Schema = &tableschema.SpannerTableSchema{
			Columns: columns,
		}
	}

	// Populate interleave if any
	table.Interleave = tableInterleaveToSchema(plan.Interleave)

	// Create table
	_, err := r.config.SpannerService.CreateSpannerTable(ctx,
		names.DatabaseName{Project: project, Instance: instanceName, Database: databaseId}.String(),
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
		names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String(),
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
			// INFORMATION_SCHEMA answers every boolean explicitly; collapse
			// hydrated false back to unset where the prior state left the
			// attribute unset, so refresh doesn't flag phantom diffs on
			// booleans the config omitted.
			if state.Schema != nil {
				priorColumns, d := tableColumnsToSchema(ctx, state.Schema.Columns)
				diags.Append(d...)
				if !d.HasError() {
					tableschema.PreserveUnsetBooleans(priorColumns, table.Schema.Columns)
				}
			}

			generatedList, d := tableColumnsToModel(ctx, table.Schema.Columns)
			diags.Append(d...)

			s.Columns = generatedList
		}

		state.Schema = s
	}

	// Populate interleave
	if table.Interleave != nil {
		state.Interleave = tableInterleaveToModel(table.Interleave)
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

// Update applies in-place schema changes and updates the Terraform state on
// success. Only schema.columns can be altered in place (the field mask passed
// below); every other attribute carries a RequiresReplace plan modifier, so
// any other change replaces the table instead of reaching this method.
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
	table := &tableschema.SpannerTable{
		Name: names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String(),
		Schema: &tableschema.SpannerTableSchema{
			Columns: nil,
		},
	}

	// Populate schema if any
	if plan.Schema != nil {
		columns, d := tableColumnsToSchema(ctx, plan.Schema.Columns)
		if d.HasError() {
			tflog.Error(ctx, fmt.Sprintf("Error reading columns: %v", d))
			return
		}
		table.Schema = &tableschema.SpannerTableSchema{
			Columns: columns,
		}
	}

	// Populate interleave if any
	table.Interleave = tableInterleaveToSchema(plan.Interleave)

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
	_, err := r.config.SpannerService.DeleteSpannerTable(ctx, names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Table",
			"Could not delete Table ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *spannerTableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importName, err := names.ParseTable(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}: "+err.Error(),
		)
		return
	}

	if !regexp.MustCompile(utils.SpannerGoogleSqlTableIdRegex).MatchString(req.ID) && !regexp.MustCompile(utils.SpannerPostgresSqlTableIdRegex).MatchString(req.ID) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be a valid Spanner Table ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions",
		)
		return
	}

	project := importName.Project
	instanceName := importName.Instance
	databaseName := importName.Database
	tableName := importName.Table

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), tableName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerTableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

// ValidateConfig emits warnings (not errors) for incomplete column
// configuration: a missing schema or columns block, PROTO columns without
// proto_package, and computed columns without computation_ddl. Warnings keep
// configs with unknown values plannable while still flagging likely mistakes.
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
		// If column type is PROTO, check if proto_package is provided. The
		// proto bundle itself must already exist in the database; the provider
		// does not manage bundles.
		if column.Type.ValueString() == "PROTO" {
			if column.ProtoPackage.IsNull() {
				resp.Diagnostics.AddAttributeWarning(
					path.Root("schema.columns").AtListIndex(i).AtName("proto_package"),
					"Missing Column Configuration",
					"Expected proto_package to be configured for columns of type PROTO. "+
						"The resource may return unexpected results.",
				)
			}
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

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
