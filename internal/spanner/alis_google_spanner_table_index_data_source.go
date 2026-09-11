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
	_ datasource.DataSource              = &tableIndexDataSource{}
	_ datasource.DataSourceWithConfigure = &tableIndexDataSource{}
)

// NewTableIndexDataSource is a helper function to simplify the provider implementation.
func NewTableIndexDataSource() datasource.DataSource {
	return &tableIndexDataSource{}
}

type tableIndexDataSource struct {
	service *services.SpannerService
}

// tableIndexDataModel reuses spannerTableIndexColumn for the columns list.
type tableIndexDataModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Database types.String `tfsdk:"database"`
	Table    types.String `tfsdk:"table"`
	Name     types.String `tfsdk:"name"`
	Columns  types.List   `tfsdk:"columns"`
	Unique   types.Bool   `tfsdk:"unique"`
}

// Metadata returns the data source type name.
func (d *tableIndexDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_index"
}

// Schema defines the schema for the data source.
func (d *tableIndexDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
			"table": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The table the index belongs to.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The index name.",
			},
			"columns": schema.ListNestedAttribute{
				Computed: true,
				CustomType: types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: spannerTableIndexColumn{}.attrTypes(),
					},
				},
				MarkdownDescription: "The indexed columns in key order.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The column name.",
						},
						"order": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The sort order of the column in the index: `asc` or `desc`.",
						},
					},
				},
			},
			"unique": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the index enforces uniqueness.",
			},
		},
		MarkdownDescription: "Reads a secondary index of a Cloud Spanner table by name. An index that does not exist is an error. " +
			"See https://cloud.google.com/spanner/docs/secondary-indexes",
	}
}

// Read fetches the index; any failure, including NotFound, is an error.
func (d *tableIndexDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state tableIndexDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tableName := names.TableName{
		Project:  state.Project.ValueString(),
		Instance: state.Instance.ValueString(),
		Database: state.Database.ValueString(),
		Table:    state.Table.ValueString(),
	}.String()
	indexName := state.Name.ValueString()

	index, err := d.service.GetSpannerTableIndex(ctx, tableName, indexName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Index",
			"Could not read Index ("+indexName+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}
	state.Unique = types.BoolValue(index.Unique.GetValue())
	columns := make([]*spannerTableIndexColumn, 0, len(index.Columns))
	for _, column := range index.Columns {
		columns = append(columns, &spannerTableIndexColumn{
			Name:  types.StringValue(column.Name),
			Order: types.StringValue(column.Order.String()),
		})
	}
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: spannerTableIndexColumn{}.attrTypes()}, columns)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Columns = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *tableIndexDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
