package spanner

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	sequenceschema "terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &databaseSequenceDataSource{}
	_ datasource.DataSourceWithConfigure = &databaseSequenceDataSource{}
)

// NewDatabaseSequenceDataSource is a helper function to simplify the provider implementation.
func NewDatabaseSequenceDataSource() datasource.DataSource {
	return &databaseSequenceDataSource{}
}

type databaseSequenceDataSource struct {
	service *services.SpannerService
}

// databaseSequenceDataModel reuses the resource's option models for the
// nested options object.
type databaseSequenceDataModel struct {
	Project  types.String            `tfsdk:"project"`
	Instance types.String            `tfsdk:"instance"`
	Database types.String            `tfsdk:"database"`
	Sequence types.String            `tfsdk:"sequence"`
	Options  *spannerSequenceOptions `tfsdk:"options"`
}

// Metadata returns the data source type name.
func (d *databaseSequenceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database_sequence"
}

// Schema defines the schema for the data source.
func (d *databaseSequenceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Google Cloud project ID containing the Spanner instance and database.",
			},
			"instance": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Spanner instance ID that contains the database.",
			},
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Spanner database ID that contains the sequence.",
			},
			"sequence": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The sequence name.",
			},
			"options": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The sequence options as stored in Spanner.",
				Attributes: map[string]schema.Attribute{
					"sequence_kind": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The sequence algorithm, for example `bit_reversed_positive`.",
					},
					"skip_range": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "The inclusive range of values the sequence never assigns, when set.",
						Attributes: map[string]schema.Attribute{
							"min": schema.Int64Attribute{
								Computed:            true,
								MarkdownDescription: "Start of the skipped range.",
							},
							"max": schema.Int64Attribute{
								Computed:            true,
								MarkdownDescription: "End of the skipped range.",
							},
						},
					},
					"start_with_counter": schema.Int64Attribute{
						Computed:            true,
						MarkdownDescription: "The sequence counter start value, when set.",
					},
				},
			},
		},
		MarkdownDescription: "Reads a Cloud Spanner sequence and its options. A sequence that does not exist is an error. " +
			"See https://cloud.google.com/spanner/docs/sequence-tasks",
	}
}

// Read fetches the sequence; any failure, including NotFound, is an error.
func (d *databaseSequenceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseSequenceDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sequenceName := names.SequenceName{
		Project:  state.Project.ValueString(),
		Instance: state.Instance.ValueString(),
		Database: state.Database.ValueString(),
		Sequence: state.Sequence.ValueString(),
	}.String()

	sequence, err := d.service.GetSpannerSequence(ctx, sequenceName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Database Sequence",
			"Could not read Sequence ("+sequenceName+"): "+utils.ErrDetail(err),
		)
		return
	}
	if sequence.Options != nil {
		options := &spannerSequenceOptions{}
		if sequence.Options.SequenceKind != sequenceschema.SpannerSequenceKindUnspecified {
			options.SequenceKind = types.StringValue(sequence.Options.SequenceKind.String())
		}
		if sequence.Options.SkipRange != nil {
			options.SkipRange = &spannerSequenceSkipRange{
				Min: types.Int64Value(sequence.Options.SkipRange.Min.GetValue()),
				Max: types.Int64Value(sequence.Options.SkipRange.Max.GetValue()),
			}
		}
		if sequence.Options.StartWithCounter != nil {
			options.StartWithCounter = types.Int64Value(sequence.Options.StartWithCounter.GetValue())
		}
		state.Options = options
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *databaseSequenceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
