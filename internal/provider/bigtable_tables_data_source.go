package provider

import (
	"context"
	"fmt"
	"strings"

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
	Name                  types.String                              `tfsdk:"name"`
	Project               types.String                              `tfsdk:"project"`
	InstanceName          types.String                              `tfsdk:"instance_name"`
	SplitKeys             []types.String                            `tfsdk:"split_keys"`
	DeletionProtection    types.Bool                                `tfsdk:"deletion_protection"`
	ChangeStreamRetention types.String                              `tfsdk:"change_stream_retention"`
	ColumnFamilies        map[string]bigtableTableColumnFamilyModel `tfsdk:"column_families"`
}

type bigtableTableColumnFamilyModel struct {
	Name                    types.String                                          `tfsdk:"name"`
	GarbageCollectionPolicy bigtableTableColumnFamilyGarbageCollectionPolicyModel `tfsdk:"garbage_collection_policy"`
}

type bigtableTableColumnFamilyGarbageCollectionPolicyModel struct {
	DeletionPolicy types.String `tfsdk:"deletion_policy"`
}

// Metadata returns the data source type name.
func (d *bigtableTablesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bigtable_tables"
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
						"project": schema.StringAttribute{
							Computed: true,
						},
						"instance_name": schema.StringAttribute{
							Computed: true,
						},
						"split_keys": schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"deletion_protection": schema.BoolAttribute{
							Computed: true,
							Optional: true,
						},
						"change_stream_retention": schema.StringAttribute{
							Optional: true,
						},
						"column_families": schema.MapNestedAttribute{
							Optional: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
									},
									"garbage_collection_policy": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"deletion_policy": schema.StringAttribute{
												Computed: true,
												Optional: true,
											},
										},
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

		// Get split keys
		var splitKeys []types.String
		// Populate split keys if any
		if table.GetSplitKeys() != nil && len(table.GetSplitKeys()) > 0 {
			for _, splitKey := range table.GetSplitKeys() {
				splitKeys = append(splitKeys, types.StringValue(splitKey))
			}
		}

		// Get deletion protection
		var deletionProtection types.Bool
		// Populate deletion protection if any
		switch table.GetDeletionProtection() {
		case pb.Table_DELETION_PROTECTION_UNSPECIFIED:
			deletionProtection = types.BoolValue(false)
		case pb.Table_PROTECTED:
			deletionProtection = types.BoolValue(true)
		case pb.Table_UNPROTECTED:
			deletionProtection = types.BoolValue(false)
		}

		// Get change stream retention
		var changeStreamRetention types.String
		// Populate change stream retention if any
		if table.GetChangeStreamRetention() != nil {
			changeStreamRetention = types.StringValue(table.GetChangeStreamRetention().AsDuration().String())
		} else {
			changeStreamRetention = types.StringValue("0s")
		}

		// Get column families
		var columnFamilies map[string]bigtableTableColumnFamilyModel
		// Populate column families if any
		if table.GetColumnFamilies() != nil && len(table.GetColumnFamilies()) > 0 {
			columnFamilies = map[string]bigtableTableColumnFamilyModel{}
			for columnFamilyName, columnFamily := range table.GetColumnFamilies() {
				// Get deletion policy
				var deletionPolicy types.String
				// Populate deletion policy if any
				switch columnFamily.GetGcPolicy().GetDeletionPolicy() {
				case pb.Table_ColumnFamily_GarbageCollectionPolicy_ABANDON:
					deletionPolicy = types.StringValue("ABANDON")
				}

				// Populate column family
				columnFamilies[columnFamilyName] = bigtableTableColumnFamilyModel{
					Name: types.StringValue(columnFamilyName),
					GarbageCollectionPolicy: bigtableTableColumnFamilyGarbageCollectionPolicyModel{
						DeletionPolicy: deletionPolicy,
					},
				}
			}
		}

		// Deconstruct table name to get project, instance, and table id
		// projects/{project}/instances/{instance}/tables/{table}
		tableNameParts := strings.Split(table.GetName(), "/")
		project := tableNameParts[1]
		instanceName := tableNameParts[3]
		tableName := tableNameParts[5]

		tableState := bigtableTableModel{
			Name:                  types.StringValue(tableName),
			Project:               types.StringValue(project),
			InstanceName:          types.StringValue(instanceName),
			SplitKeys:             splitKeys,
			DeletionProtection:    deletionProtection,
			ChangeStreamRetention: changeStreamRetention,
			ColumnFamilies:        columnFamilies,
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
