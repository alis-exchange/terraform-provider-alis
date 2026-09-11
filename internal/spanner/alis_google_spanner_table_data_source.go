package spanner

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &spannerTableDataSource{}
	_ datasource.DataSourceWithConfigure = &spannerTableDataSource{}
)

// NewSpannerTableDataSource is a helper function to simplify the provider implementation.
func NewSpannerTableDataSource() datasource.DataSource {
	return &spannerTableDataSource{}
}

type spannerTableDataSource struct {
	service *services.SpannerService
}

// spannerTableDataModel reuses the resource's nested models so the hydrated
// columns and interleave come straight from tableColumnsToModel and
// tableInterleaveToModel. There is no prior state to reconcile against, so
// values are stored exactly as the database reports them.
type spannerTableDataModel struct {
	Project    types.String            `tfsdk:"project"`
	Instance   types.String            `tfsdk:"instance"`
	Database   types.String            `tfsdk:"database"`
	Name       types.String            `tfsdk:"name"`
	Schema     *spannerTableSchema     `tfsdk:"schema"`
	Interleave *spannerTableInterleave `tfsdk:"interleave"`
}

// Metadata returns the data source type name.
func (d *spannerTableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table"
}

// Schema defines the schema for the data source.
func (d *spannerTableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
				MarkdownDescription: "The Spanner database ID that contains the table.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The table name.",
			},
			"schema": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The table schema as hydrated from INFORMATION_SCHEMA.",
				Attributes: map[string]schema.Attribute{
					"columns": schema.ListNestedAttribute{
						Computed: true,
						CustomType: types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spannerTableColumn{}.attrTypes(),
							},
						},
						MarkdownDescription: "The columns in ordinal order.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The column name.",
								},
								"is_primary_key": schema.BoolAttribute{
									Computed:            true,
									MarkdownDescription: "Whether the column is part of the primary key.",
								},
								"is_computed": schema.BoolAttribute{
									Computed:            true,
									MarkdownDescription: "Whether the column is a generated column.",
								},
								"computation_ddl": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The generation expression of a generated column.",
								},
								"is_stored": schema.BoolAttribute{
									Computed:            true,
									MarkdownDescription: "Whether a generated column is stored.",
								},
								"auto_update_time": schema.BoolAttribute{
									Computed:            true,
									MarkdownDescription: "Whether the column is a commit-timestamp column (`allow_commit_timestamp`).",
								},
								"type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The column data type, for example `INT64`, `STRING`, `PROTO` or `ARRAY<STRING>`.",
								},
								"size": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: "The declared length of a `STRING` or `BYTES` column, when bounded.",
								},
								"required": schema.BoolAttribute{
									Computed:            true,
									MarkdownDescription: "Whether the column is `NOT NULL`.",
								},
								"default_value": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The column's `DEFAULT (...)` expression, when set.",
								},
								"proto_package": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The fully qualified proto message or enum of a `PROTO` column.",
								},
							},
						},
					},
				},
			},
			"interleave": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The parent table this table is interleaved in; null when the table is not interleaved.",
				Attributes: map[string]schema.Attribute{
					"parent_table": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The parent table name.",
					},
					"on_delete": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The action on parent row deletion: `CASCADE` or `NO ACTION`.",
					},
				},
			},
		},
		MarkdownDescription: "Reads a Cloud Spanner table's columns and interleaving as they exist in the database. A table that does not exist is an error. " +
			"Use it to reference tables managed elsewhere, for example to build indexes or foreign keys against them.",
	}
}

// Read hydrates the table; any failure, including NotFound, is an error.
func (d *spannerTableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state spannerTableDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tableName := names.TableName{
		Project:  state.Project.ValueString(),
		Instance: state.Instance.ValueString(),
		Database: state.Database.ValueString(),
		Table:    state.Name.ValueString(),
	}.String()

	table, err := d.service.GetSpannerTable(ctx, tableName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Table",
			"Could not read Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}
	if table.Schema != nil {
		columns, diags := tableColumnsToModel(ctx, table.Schema.Columns)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Schema = &spannerTableSchema{Columns: columns}
	}
	state.Interleave = tableInterleaveToModel(table.Interleave)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *spannerTableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
