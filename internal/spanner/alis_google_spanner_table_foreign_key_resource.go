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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &spannerTableForeignKeyResource{}
	_ resource.ResourceWithConfigure   = &spannerTableForeignKeyResource{}
	_ resource.ResourceWithImportState = &spannerTableForeignKeyResource{}
)

// NewTableForeignKeyResource is a helper function to simplify the provider implementation.
func NewTableForeignKeyResource() resource.Resource {
	return &spannerTableForeignKeyResource{}
}

type spannerTableForeignKeyResource struct {
	config *internal.ProviderConfig
}

type spannerTableForeignKeyModel struct {
	Project          types.String   `tfsdk:"project"`
	Instance         types.String   `tfsdk:"instance"`
	Database         types.String   `tfsdk:"database"`
	Table            types.String   `tfsdk:"table"`
	Name             types.String   `tfsdk:"name"`
	ReferencedTable  types.String   `tfsdk:"referenced_table"`
	Column           types.String   `tfsdk:"column"`
	ReferencedColumn types.String   `tfsdk:"referenced_column"`
	OnDelete         types.String   `tfsdk:"on_delete"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

// Metadata returns the resource type name.
func (r *spannerTableForeignKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_foreign_key"
}

// Schema defines the schema for the resource.
func (r *spannerTableForeignKeyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
		Attributes: map[string]schema.Attribute{
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
				Description: "The name of the constrained/referencing table.\n" +
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
			"name": schema.StringAttribute{
				Required: true,
				Description: "The name of the foreign key constraint.\n" +
					"The name must satisfy the expression `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`.\n" +
					"The **FK_** prefix is recommended but not required.",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						regexp.MustCompile(utils.SpannerGoogleSqlConstraintIdRegex),
						regexp.MustCompile(utils.SpannerPostgresSqlConstraintIdRegex),
					}, "Name must be a valid Spanner Constraint ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"referenced_table": schema.StringAttribute{
				Required: true,
				Description: "The name of the referenced table.\n" +
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
			"column": schema.StringAttribute{
				Required: true,
				Description: "The name of the constrained/referencing column.\n" +
					"See https://cloud.google.com/spanner/docs/foreign-keys/overview",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						regexp.MustCompile(utils.SpannerGoogleSqlColumnIdRegex),
						regexp.MustCompile(utils.SpannerPostgresSqlColumnIdRegex),
					}, "Column must be a valid Spanner Column ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"referenced_column": schema.StringAttribute{
				Required: true,
				Description: "The name of the referenced column.\n" +
					"See https://cloud.google.com/spanner/docs/foreign-keys/overview",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						regexp.MustCompile(utils.SpannerGoogleSqlColumnIdRegex),
						regexp.MustCompile(utils.SpannerPostgresSqlColumnIdRegex),
					}, "Column must be a valid Spanner Column ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"on_delete": schema.StringAttribute{
				Required: true,
				Description: "The action to take when the referenced row is deleted.\n" +
					"Supported values are `CASCADE`, `NO_ACTION`.\n" +
					"See https://cloud.google.com/spanner/docs/foreign-keys/overview#how-to-define-foreign-key-action",
				Validators: []validator.String{
					stringvalidator.OneOf(tableschema.SpannerTableConstraintActions...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Description: "A Spanner Table Foreign Key resource. See https://cloud.google.com/spanner/docs/foreign-keys/overview",
	}
}

// Create a new resource.
func (r *spannerTableForeignKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan spannerTableForeignKeyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, createTimeout)
	defer cancel()

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	databaseId := plan.Database.ValueString()
	tableId := plan.Table.ValueString()

	// Generate policy from plan
	constraint := &tableschema.SpannerTableForeignKeyConstraint{
		Name:             plan.Name.ValueString(),
		ReferencedTable:  plan.ReferencedTable.ValueString(),
		ReferencedColumn: plan.ReferencedColumn.ValueString(),
		Column:           plan.Column.ValueString(),
		OnDelete:         tableschema.SpannerTableConstraintActionFromString(plan.OnDelete.ValueString()),
	}

	// Create row deletion policy
	_, err := r.config.SpannerService.CreateSpannerTableForeignKeyConstraint(ctx,
		names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String(),
		constraint,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Foreign Key Constraint",
			"Could not create Foreign Key Constraint: "+err.Error(),
		)
		return
	}

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
func (r *spannerTableForeignKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state spannerTableForeignKeyModel
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
	name := state.Name.ValueString()

	// Get policy from API
	constraint, err := r.config.SpannerService.GetSpannerTableForeignKeyConstraint(ctx, names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String(), name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Foreign Key Constraint",
			"Could not read Foreign Key Constraint: "+err.Error(),
		)
		return
	}

	// Set refreshed state
	state.Name = types.StringValue(constraint.Name)
	state.ReferencedTable = types.StringValue(constraint.ReferencedTable)
	state.ReferencedColumn = types.StringValue(constraint.ReferencedColumn)
	state.Column = types.StringValue(constraint.Column)
	state.OnDelete = types.StringValue(constraint.OnDelete.String())

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

func (r *spannerTableForeignKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan spannerTableForeignKeyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Every real attribute carries RequiresReplace, so reaching Update means
	// only the timeouts block changed; persist the plan without touching
	// Spanner. Erroring here would make a timeouts-only change un-applyable.
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *spannerTableForeignKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state spannerTableForeignKeyModel
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
	name := state.Name.ValueString()

	// Delete existing database
	err := r.config.SpannerService.DeleteSpannerTableForeignKeyConstraint(ctx, names.TableName{Project: project, Instance: instanceName, Database: databaseId, Table: tableId}.String(), name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Foreign Key Constraint",
			"Could not delete Foreign Key Constraint: "+err.Error(),
		)
		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *spannerTableForeignKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

// ImportState imports an existing foreign key constraint into state.
func (r *spannerTableForeignKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importName, err := names.ParseForeignKey(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{database}/tables/{table}/constraints/{constraint}: "+err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), importName.Project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), importName.Instance)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), importName.Database)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), importName.Table)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), importName.Constraint)...)
}
