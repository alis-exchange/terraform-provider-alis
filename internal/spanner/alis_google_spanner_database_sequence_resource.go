package spanner

import (
	"context"
	"regexp"

	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/names"
	sequenceschema "terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	_ resource.Resource                = &databaseSequenceResource{}
	_ resource.ResourceWithConfigure   = &databaseSequenceResource{}
	_ resource.ResourceWithImportState = &databaseSequenceResource{}
)

// NewDatabaseSequenceResource is a helper function to simplify the provider implementation.
func NewDatabaseSequenceResource() resource.Resource {
	return &databaseSequenceResource{}
}

type databaseSequenceResource struct {
	config *internal.ProviderConfig
}

type databaseSequenceModel struct {
	Project  types.String            `tfsdk:"project"`
	Instance types.String            `tfsdk:"instance"`
	Database types.String            `tfsdk:"database"`
	Sequence types.String            `tfsdk:"sequence"`
	Options  *spannerSequenceOptions `tfsdk:"options"`
	Timeouts timeouts.Value          `tfsdk:"timeouts"`
}

type spannerSequenceOptions struct {
	SequenceKind     types.String              `tfsdk:"sequence_kind"`
	SkipRange        *spannerSequenceSkipRange `tfsdk:"skip_range"`
	StartWithCounter types.Int64               `tfsdk:"start_with_counter"`
}

type spannerSequenceSkipRange struct {
	Min types.Int64 `tfsdk:"min"`
	Max types.Int64 `tfsdk:"max"`
}

// Metadata returns the resource type name.
func (r *databaseSequenceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database_sequence"
}

// Schema defines the schema for the resource.
func (r *databaseSequenceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Required: true,
				MarkdownDescription: "The Google Cloud project ID containing the Spanner instance and database.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Spanner instance ID that contains the database.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Spanner database ID within the instance where sequence DDL is applied.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"sequence": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The sequence name within the database. Referenced in SQL when using the sequence (for example GET_NEXT_SEQUENCE_VALUE with Google-standard-SQL).\n" +
					"Must satisfy Spanner sequence identifier naming rules. See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions\n" +
					"Changing this forces a new resource.",
				Validators: []validator.String{
					validators.RegexMatches([]*regexp.Regexp{
						utils.Pattern(utils.SpannerGoogleSqlSequenceIdRegex),
						utils.Pattern(utils.SpannerPostgresSqlSequenceIdRegex),
					}, "Name must be a valid Spanner Sequence ID, See https://cloud.google.com/spanner/docs/reference/standard-sql/data-definition-language#naming_conventions"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"options": schema.SingleNestedAttribute{
				Required: true,
				MarkdownDescription: "DDL options for the sequence, equivalent to OPTIONS on CREATE SEQUENCE and SET OPTIONS on ALTER SEQUENCE for Google-standard-SQL.\n" +
					"In Terraform configuration use object assignment (options = { ... }), not a nested options block. See https://cloud.google.com/spanner/docs/sequence-tasks",
				Attributes: map[string]schema.Attribute{
					"sequence_kind": schema.StringAttribute{
						Required: true,
						MarkdownDescription: "The sequence algorithm. Use bit_reversed_positive for bit-reversed positive sequences, which Spanner recommends for scalable surrogate primary keys.\n" +
							"See https://cloud.google.com/spanner/docs/sequence-tasks",
					},
					"skip_range": schema.SingleNestedAttribute{
						Optional: true,
						MarkdownDescription: "Inclusive range of integers that the sequence must not assign to new values (skip_range_min and skip_range_max in Spanner DDL).\n" +
							"When set, both min and max are required. See https://cloud.google.com/spanner/docs/sequence-tasks",
						Attributes: map[string]schema.Attribute{
							"min": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "Start of the inclusive skip range; maps to skip_range_min in Spanner sequence OPTIONS.",
							},
							"max": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "End of the inclusive skip range; maps to skip_range_max in Spanner sequence OPTIONS.",
							},
						},
					},
					"start_with_counter": schema.Int64Attribute{
						Optional: true,
						MarkdownDescription: "Sets the sequence counter (start_with_counter in Spanner OPTIONS). Changing this affects future generated values; review operational impact before updating.\n" +
							"See https://cloud.google.com/spanner/docs/sequence-tasks",
					},
				},
			},
		},
		MarkdownDescription: "Manages a Cloud Spanner database sequence using DDL. If the sequence does not exist it is created; if it already exists it is imported into Terraform state.\n" +
			"Updates apply ALTER SEQUENCE ... SET OPTIONS for Google-standard-SQL. Binding a sequence to table columns (defaults) and dropping a sequence are separate DDL or console steps; see https://cloud.google.com/spanner/docs/sequence-tasks",
	}
}

// Create ensures the sequence exists: a sequence already present in the
// database is adopted into state as-is rather than treated as a conflict;
// otherwise CREATE SEQUENCE DDL is issued with the planned options.
func (r *databaseSequenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan databaseSequenceModel
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

	// Retrieve project, instance and database from state
	project := plan.Project.ValueString()
	instance := plan.Instance.ValueString()
	databaseId := plan.Database.ValueString()
	sequenceId := plan.Sequence.ValueString()

	sequenceName := names.SequenceName{Project: project, Instance: instance, Database: databaseId, Sequence: sequenceId}.String()

	existingSequence, err := r.config.SpannerService.GetSpannerSequence(ctx, sequenceName)
	if err != nil && status.Code(err) != codes.NotFound {
		resp.Diagnostics.AddError(
			"Error Checking Existing Database Sequence",
			"Could not verify whether Sequence ("+sequenceName+") already exists: "+utils.ErrDetail(err),
		)
		return
	}
	if existingSequence != nil {
		// Set state to fully populated data
		diags = resp.State.Set(ctx, plan)
		resp.Diagnostics.Append(diags...)
		return
	}

	// Create sequence from plan
	sequence := &sequenceschema.SpannerSequence{
		Name: sequenceName,
	}

	// Populate options if any
	if plan.Options != nil {
		sequenceOptions := &sequenceschema.SpannerSequenceOptions{
			SequenceKind: sequenceschema.SpannerSequenceKindBitReversedPositive,
		}

		if !plan.Options.SequenceKind.IsNull() {
			sequenceOptions.SequenceKind = sequenceschema.SpannerSequenceKindFromString(plan.Options.SequenceKind.ValueString())
		}

		if plan.Options.SkipRange != nil {
			sequenceOptions.SkipRange = &sequenceschema.SpannerSequenceSkipRange{}
			if !plan.Options.SkipRange.Min.IsNull() {
				sequenceOptions.SkipRange.Min = wrapperspb.Int64(plan.Options.SkipRange.Min.ValueInt64())
			}
			if !plan.Options.SkipRange.Max.IsNull() {
				sequenceOptions.SkipRange.Max = wrapperspb.Int64(plan.Options.SkipRange.Max.ValueInt64())
			}
		}

		if !plan.Options.StartWithCounter.IsNull() {
			sequenceOptions.StartWithCounter = wrapperspb.Int64(plan.Options.StartWithCounter.ValueInt64())
		}

		sequence.Options = sequenceOptions
	}

	_, err = r.config.SpannerService.CreateSpannerSequence(ctx,
		names.DatabaseName{Project: project, Instance: instance, Database: databaseId}.String(),
		sequence,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Database Sequence",
			"Could not create Sequence ("+sequenceName+"): "+utils.ErrDetail(err),
		)
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
func (r *databaseSequenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state databaseSequenceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	databaseId := state.Database.ValueString()
	sequenceId := state.Sequence.ValueString()

	sequenceName := names.SequenceName{Project: project, Instance: instance, Database: databaseId, Sequence: sequenceId}.String()

	sequence, err := r.config.SpannerService.GetSpannerSequence(ctx, sequenceName)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Database Sequence",
			"Could not read Sequence ("+sequenceName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Populate state from sequence
	state.Sequence = types.StringValue(sequenceId)

	if sequence.Options != nil {
		options := &spannerSequenceOptions{}

		if sequence.Options.SequenceKind != sequenceschema.SpannerSequenceKindUnspecified {
			options.SequenceKind = types.StringValue(sequence.Options.SequenceKind.String())
		}

		if sequence.Options.SkipRange != nil {
			// GetValue is nil-safe; either bound may be unset.
			options.SkipRange = &spannerSequenceSkipRange{
				Min: types.Int64Value(sequence.Options.SkipRange.Min.GetValue()),
				Max: types.Int64Value(sequence.Options.SkipRange.Max.GetValue()),
			}
		}

		if sequence.Options.StartWithCounter != nil {
			options.StartWithCounter = types.Int64Value(sequence.Options.StartWithCounter.Value)
		}

		state.Options = options
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *databaseSequenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan databaseSequenceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, updateTimeout)
	defer cancel()

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
	databaseId := plan.Database.ValueString()
	sequenceId := plan.Sequence.ValueString()

	sequenceName := names.SequenceName{Project: project, Instance: instanceName, Database: databaseId, Sequence: sequenceId}.String()

	// Generate sequence from plan
	sequence := &sequenceschema.SpannerSequence{
		Name: sequenceName,
	}

	// Populate options if any
	if plan.Options != nil {
		sequenceOptions := &sequenceschema.SpannerSequenceOptions{
			SequenceKind: sequenceschema.SpannerSequenceKindBitReversedPositive,
		}

		if !plan.Options.SequenceKind.IsNull() {
			sequenceOptions.SequenceKind = sequenceschema.SpannerSequenceKindFromString(plan.Options.SequenceKind.ValueString())
		}

		if plan.Options.SkipRange != nil {
			sequenceOptions.SkipRange = &sequenceschema.SpannerSequenceSkipRange{}
			if !plan.Options.SkipRange.Min.IsNull() {
				sequenceOptions.SkipRange.Min = wrapperspb.Int64(plan.Options.SkipRange.Min.ValueInt64())
			}
			if !plan.Options.SkipRange.Max.IsNull() {
				sequenceOptions.SkipRange.Max = wrapperspb.Int64(plan.Options.SkipRange.Max.ValueInt64())
			}
		}

		if !plan.Options.StartWithCounter.IsNull() {
			sequenceOptions.StartWithCounter = wrapperspb.Int64(plan.Options.StartWithCounter.ValueInt64())
		}

		sequence.Options = sequenceOptions
	}

	_, err := r.config.SpannerService.UpdateSpannerSequence(ctx, sequence)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Database Sequence",
			"Could not update Sequence ("+sequenceName+"): "+utils.ErrDetail(err),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Sequence = types.StringValue(sequenceId)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *databaseSequenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state databaseSequenceModel
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

	// Retrieve project, instance and database from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	database := state.Database.ValueString()
	sequenceId := state.Sequence.ValueString()

	sequenceName := names.SequenceName{Project: project, Instance: instance, Database: database, Sequence: sequenceId}.String()

	err := r.config.SpannerService.DeleteSpannerSequence(ctx, sequenceName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Database Sequence",
			"Could not delete Sequence ("+sequenceName+"): "+utils.ErrDetail(err),
		)
		return
	}
}

func (r *databaseSequenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{database}/sequences/{sequence}
	importName, err := names.ParseSequence(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") must be in the format projects/{project}/instances/{instance}/databases/{database}/sequences/{sequence}: "+err.Error(),
		)
		return
	}
	project := importName.Project
	instanceName := importName.Instance
	databaseName := importName.Database
	sequenceId := importName.Sequence

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), databaseName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sequence"), sequenceId)...)
}

// Configure adds the provider configured client to the resource.
func (r *databaseSequenceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	config, ok := configureProviderConfig(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.config = config
}

func (r *databaseSequenceResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
