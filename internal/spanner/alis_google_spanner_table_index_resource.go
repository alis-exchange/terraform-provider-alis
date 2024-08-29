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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"
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
	Name     types.String `tfsdk:"name"`
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Database types.String `tfsdk:"database"`
	Table    types.String `tfsdk:"table"`
	Columns  types.List   `tfsdk:"columns"`
	Unique   types.Bool   `tfsdk:"unique"`
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
func (r *spannerTableIndexResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_index"
}

// Schema defines the schema for the resource.
func (r *spannerTableIndexResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
			"table": schema.StringAttribute{
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
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
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
		Description: "A Google Cloud Spanner table index resource.\n" +
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
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}

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
		index.Columns = append(index.Columns, &services.SpannerTableIndexColumn{
			Name:  column.Name.ValueString(),
			Order: order,
		})
	}

	if !plan.Unique.IsNull() {
		index.Unique = wrapperspb.Bool(plan.Unique.ValueBool())
	}

	// Create table
	_, err := r.config.SpannerService.CreateSpannerTableIndex(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
		index,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Index",
			"Could not create Index ("+plan.Name.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(indexName)

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
func (r *spannerTableIndexResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state spannerTableIndexModel
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
	tableId := state.Table.ValueString()
	indexName := state.Name.ValueString()

	// Get table from API
	index, err := r.config.SpannerService.GetSpannerTableIndex(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
		indexName,
	)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Index",
			"Could not read Index ("+state.Name.ValueString()+"): "+err.Error(),
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
	diags.Append(d...)
	state.Columns = generatedList

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

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.Instance.ValueString()
	databaseId := state.Database.ValueString()
	tableId := state.Table.ValueString()
	indexName := state.Name.ValueString()

	// Delete existing database
	_, err := r.config.SpannerService.DeleteSpannerTableIndex(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", project, instanceName, databaseId, tableId),
		indexName,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Index",
			"Could not delete Index ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *spannerTableIndexResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{database}/tables/{tables}/indexes/{index}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 10 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}/indexes/{index}",
		)
		return
	}

	if !regexp.MustCompile(utils.SpannerGoogleSqlTableIndexNameRegex).MatchString(req.ID) && !regexp.MustCompile(utils.SpannerPostgresSqlTableIndexNameRegex).MatchString(req.ID) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}/indexes/{index}",
		)
		return
	}

	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]
	tableName := importIDParts[7]
	indexName := importIDParts[9]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), tableName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), indexName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerTableIndexResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *spannerTableIndexResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
