package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &bigtableTablesDataSource{}
	_ datasource.DataSourceWithConfigure = &bigtableTablesDataSource{}
)

func NewBigtableTablesDataSource() datasource.DataSource {
	return &bigtableTablesDataSource{}
}

type bigtableTablesDataSource struct {
	client pb.BigtableServiceClient
}

type bigtableTablesDataSourceModel struct {
	Tables []bigtableTableModel `tfsdk:"tables"`
}

type bigtableTableModel struct {
	Name           types.String                              `tfsdk:"name"`
	ColumnFamilies map[string]bigtableTableColumnFamilyModel `tfsdk:"column_families"`
}

type bigtableTableColumnFamilyModel struct {
	Name types.String `tfsdk:"name"`
}

// Metadata returns the data source type name.
func (d *bigtableTablesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tables"
}

// Schema defines the schema for the data source.
func (d *bigtableTablesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"tables": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed: true,
						},
						"column_families": schema.MapNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Computed: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *bigtableTablesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state bigtableTablesDataSourceModel

	tablesRes, err := d.client.ListTables(ctx, &pb.ListTablesRequest{})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read DB Tables",
			err.Error(),
		)
		return
	}

	// Map response body to model
	for _, table := range tablesRes.GetTables() {
		tableState := bigtableTableModel{
			Name:           types.StringValue(table.GetName()),
			ColumnFamilies: map[string]bigtableTableColumnFamilyModel{},
		}

		// Populate column families if any
		if table.GetColumnFamilies() != nil && len(table.GetColumnFamilies()) > 0 {
			for columnFamilyName := range table.GetColumnFamilies() {
				tableState.ColumnFamilies[columnFamilyName] = bigtableTableColumnFamilyModel{
					Name: types.StringValue(columnFamilyName),
				}
			}
		}

		state.Tables = append(state.Tables, tableState)
	}

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *bigtableTablesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(pb.BigtableServiceClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected pb.BigtableServiceClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}
