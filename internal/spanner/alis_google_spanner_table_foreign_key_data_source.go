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
	_ datasource.DataSource              = &tableForeignKeyDataSource{}
	_ datasource.DataSourceWithConfigure = &tableForeignKeyDataSource{}
)

// NewTableForeignKeyDataSource is a helper function to simplify the provider implementation.
func NewTableForeignKeyDataSource() datasource.DataSource {
	return &tableForeignKeyDataSource{}
}

type tableForeignKeyDataSource struct {
	service *services.SpannerService
}

type tableForeignKeyDataModel struct {
	Project          types.String `tfsdk:"project"`
	Instance         types.String `tfsdk:"instance"`
	Database         types.String `tfsdk:"database"`
	Table            types.String `tfsdk:"table"`
	Name             types.String `tfsdk:"name"`
	ReferencedTable  types.String `tfsdk:"referenced_table"`
	Column           types.String `tfsdk:"column"`
	ReferencedColumn types.String `tfsdk:"referenced_column"`
	OnDelete         types.String `tfsdk:"on_delete"`
}

// Metadata returns the data source type name.
func (d *tableForeignKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_foreign_key"
}

// Schema defines the schema for the data source.
func (d *tableForeignKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
				MarkdownDescription: "The constrained (referencing) table.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The foreign key constraint name.",
			},
			"referenced_table": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The table the constraint references.",
			},
			"column": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The constrained column in `table`.",
			},
			"referenced_column": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The referenced column in `referenced_table`.",
			},
			"on_delete": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The referential action on delete: `CASCADE` or `NO ACTION`.",
			},
		},
		MarkdownDescription: "Reads a foreign key constraint of a Cloud Spanner table by name. A constraint that does not exist is an error. " +
			"See https://cloud.google.com/spanner/docs/foreign-keys/overview",
	}
}

// Read fetches the constraint; any failure, including NotFound, is an error.
func (d *tableForeignKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state tableForeignKeyDataModel
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
	name := state.Name.ValueString()

	constraint, err := d.service.GetSpannerTableForeignKeyConstraint(ctx, tableName, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Foreign Key Constraint",
			"Could not read Foreign Key Constraint ("+name+") on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}
	state.ReferencedTable = types.StringValue(constraint.ReferencedTable)
	state.ReferencedColumn = types.StringValue(constraint.ReferencedColumn)
	state.Column = types.StringValue(constraint.Column)
	state.OnDelete = types.StringValue(constraint.OnDelete.String())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *tableForeignKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
