package spanner

import (
	"context"
	"regexp"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &spannerTableIndexResource{}
	_ resource.ResourceWithConfigure   = &spannerTableIndexResource{}
	_ resource.ResourceWithImportState = &spannerTableIndexResource{}
)

// NewSpannerTableIndexResource is a helper function to simplify the provider implementation.
func NewSpannerTableIndexResource() resource.Resource {
	return &spannerTableIndexResource{}
}

type spannerTableIndexResource struct {
	config *internal.ProviderConfig
}

type spannerTableIndexModel struct {
	Name     types.String   `tfsdk:"name"`
	Project  types.String   `tfsdk:"project"`
	Instance types.String   `tfsdk:"instance"`
	Database types.String   `tfsdk:"database"`
	Table    types.String   `tfsdk:"table"`
	Columns  types.List     `tfsdk:"columns"`
	Unique   types.Bool     `tfsdk:"unique"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
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

// resolveUnknownIndexInherited replaces unknown unique and columns.order
// values with null before a plan is persisted as state. Both are Computed, so
// omitted values plan as unknown, and a brand-new index (or newly added
// column) has no prior state for UseStateForUnknown to inherit — but unknown
// values cannot be stored. The next Read hydrates the real values.
func resolveUnknownIndexInherited(ctx context.Context, plan spannerTableIndexModel) (spannerTableIndexModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	if plan.Unique.IsUnknown() {
		plan.Unique = types.BoolNull()
	}
	if plan.Columns.IsNull() || plan.Columns.IsUnknown() {
		return plan, diags
	}

	columns := make([]spannerTableIndexColumn, 0, len(plan.Columns.Elements()))
	diags.Append(plan.Columns.ElementsAs(ctx, &columns, false)...)
	if diags.HasError() {
		return plan, diags
	}
	for i := range columns {
		if columns[i].Order.IsUnknown() {
			columns[i].Order = types.StringNull()
		}
	}
	resolved, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: spannerTableIndexColumn{}.attrTypes()}, columns)
	diags.Append(d...)
	if diags.HasError() {
		return plan, diags
	}
	plan.Columns = resolved
	return plan, diags
}

// Metadata returns the resource type name.
func (r *spannerTableIndexResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_index"
}

// Schema defines the schema for the resource.
func (r *spannerTableIndexResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: resourceSchemaVersion,
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The name of the index.\n" +
					"The name must contain only letters (a-z, A-Z), numbers (0-9), or hyphens (-), and must start with a letter and not end in a hyphen.",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						utils.Pattern(utils.SpannerGoogleSqlIndexIdRegex),
						utils.Pattern(utils.SpannerPostgresSqlIndexIdRegex),
					}, "Name must be a valid Spanner Index ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Google Cloud project ID in which the table belongs.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the Spanner instance.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the parent database.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"table": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The name of the table.\n" +
					"The name must satisfy the expression `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						utils.Pattern(utils.SpannerGoogleSqlTableIdRegex),
						utils.Pattern(utils.SpannerPostgresSqlTableIdRegex),
					}, "Name must be a valid Spanner Table ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
							Required:            true,
							MarkdownDescription: "The name of the column that makes up the index.",
							Validators: []validator.String{
								validators.RegexMatches([]*regexp.Regexp{
									utils.Pattern(utils.SpannerGoogleSqlColumnIdRegex),
									utils.Pattern(utils.SpannerPostgresSqlColumnIdRegex),
								}, "Name must be a valid Spanner Column ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
							},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"order": schema.StringAttribute{
							Optional: true,
							// Computed with UseStateForUnknown: state always holds the
							// hydrated order ("asc" when omitted), so unset config must
							// inherit it instead of reading as a change that recreates
							// the index.
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							MarkdownDescription: "The sorting order of the column in the index.\n" +
								"Valid values are: `asc` or `desc`. If not specified the default is `asc`.\n" +
								"When omitted, the column's current order in the database is kept.",
							Validators: []validator.String{
								stringvalidator.OneOf(services.SpannerTableIndexColumnOrders...),
							},
						},
					},
				},
				MarkdownDescription: "The columns that make up the index.\n" +
					"The order of the columns is significant.\n" +
					"**Changing any column will destroy and recreate the index**: Spanner indexes cannot be altered in place.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"unique": schema.BoolAttribute{
				Optional: true,
				// Computed with UseStateForUnknown, which must run before
				// RequiresReplace: state always holds the hydrated value
				// (false when omitted), so unset config inherits it and the
				// replace check compares real values, never unknown.
				Computed: true,
				MarkdownDescription: "Indicates if the index is unique.\n" +
					"When omitted, the index's current uniqueness in the database is kept.\n" +
					"**Changing this value explicitly will destroy and recreate the index**: Spanner indexes cannot be altered in place.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
		MarkdownDescription: "A Google Cloud Spanner table index resource.\n" +
			"This resource manages the indexes on a table in a Google Cloud Spanner database.",
	}
}

// Create a new resource.
func (r *spannerTableIndexResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan spannerTableIndexModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, createTimeout)
	defer cancel()

	// Generate index from plan
	index := &services.SpannerTableIndex{
		Name:    plan.Name.ValueString(),
		Columns: []*services.SpannerTableIndexColumn{},
		Unique:  nil,
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	databaseId := plan.Database.ValueString()
	tableId := plan.Table.ValueString()
	indexName := plan.Name.ValueString()

	columns := make([]spannerTableIndexColumn, 0, len(plan.Columns.Elements()))
	d := plan.Columns.ElementsAs(ctx, &columns, false)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, column := range columns {
		order := services.SpannerTableIndexColumnOrder_ASC
		switch column.Order.ValueString() {
		case "asc":
			order = services.SpannerTableIndexColumnOrder_ASC
		case "desc":
			order = services.SpannerTableIndexColumnOrder_DESC
		}
		index.Columns = append(index.Columns, &services.SpannerTableIndexColumn{
			Name:  column.Name.ValueString(),
			Order: order,
		})
	}

	// unique is Computed: at create time an omitted value is still unknown
	// (no prior state to inherit), which must read as unset, not false.
	if !plan.Unique.IsNull() && !plan.Unique.IsUnknown() {
		index.Unique = wrapperspb.Bool(plan.Unique.ValueBool())
	}

	tableName := names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String()

	// Create index
	_, err := r.config.SpannerService.CreateSpannerTableIndex(ctx, tableName, index)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Index",
			"Could not create Index ("+indexName+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(indexName)
	plan, rd := resolveUnknownIndexInherited(ctx, plan)
	resp.Diagnostics.Append(rd...)
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

// Read resource information.
func (r *spannerTableIndexResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state spannerTableIndexModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.Instance.ValueString()
	databaseId := state.Database.ValueString()
	tableId := state.Table.ValueString()
	indexName := state.Name.ValueString()

	// Get table from API
	index, err := r.config.SpannerService.GetSpannerTableIndex(ctx,
		names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String(),
		indexName,
	)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Index",
			"Could not read Index ("+indexName+") on Table ("+names.TableName{
				Project:  project,
				Instance: instanceName,
				Database: databaseId,
				Table:    tableId,
			}.String()+"): "+utils.ErrDetail(
				err,
			),
		)
		return
	}

	// Set refreshed state
	state.Name = types.StringValue(indexName)

	// Get unique
	if index.Unique != nil {
		state.Unique = types.BoolValue(index.Unique.GetValue())
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
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Columns = generatedList

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update exists to satisfy the resource interface but is effectively
// unreachable: a secondary index cannot be altered in place, so every
// attribute carries a RequiresReplace plan modifier and any change plans as
// a destroy-and-recreate instead of an update.
func (r *spannerTableIndexResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan spannerTableIndexModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	indexName := plan.Name.ValueString()

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(indexName)
	plan, rd := resolveUnknownIndexInherited(ctx, plan)
	resp.Diagnostics.Append(rd...)
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
func (r *spannerTableIndexResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state spannerTableIndexModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, deleteTimeout)
	defer cancel()

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.Instance.ValueString()
	databaseId := state.Database.ValueString()
	tableId := state.Table.ValueString()
	indexName := state.Name.ValueString()

	tableName := names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String()

	// Delete existing index
	_, err := r.config.SpannerService.DeleteSpannerTableIndex(ctx, tableName, indexName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Index",
			"Could not delete Index ("+indexName+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}
}

func (r *spannerTableIndexResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importName, err := names.ParseIndex(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}/indexes/{index}: "+err.Error(),
		)
		return
	}

	if !utils.Pattern(utils.SpannerGoogleSqlTableIndexNameRegex).MatchString(req.ID) &&
		!utils.Pattern(utils.SpannerPostgresSqlTableIndexNameRegex).MatchString(req.ID) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") contains an invalid project, instance, database, table or index ID. Expected format: projects/{project}/instances/{instance}/databases/{database}/tables/{table}/indexes/{index}.",
		)
		return
	}

	project := importName.Project
	instanceName := importName.Instance
	databaseName := importName.Database
	tableName := importName.Table
	indexName := importName.Index

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), tableName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), indexName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerTableIndexResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

func (r *spannerTableIndexResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
