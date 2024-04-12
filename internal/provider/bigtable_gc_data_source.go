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
	_ datasource.DataSource              = &bigtableGarbageCollectionPolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &bigtableGarbageCollectionPolicyDataSource{}
)

func NewBigtableGarbageCollectionPolicyDataSource() datasource.DataSource {
	return &bigtableGarbageCollectionPolicyDataSource{}
}

type bigtableGarbageCollectionPolicyDataSource struct {
	client pb.BigtableServiceClient
}

type bigtableGarbageCollectionPolicyModel struct {
	Project        types.String            `tfsdk:"project"`
	InstanceName   types.String            `tfsdk:"instance_name"`
	Table          types.String            `tfsdk:"table"`
	ColumFamily    types.String            `tfsdk:"column_family"`
	DeletionPolicy types.String            `tfsdk:"deletion_policy"`
	MaxVersion     bigtableMaxVersionModel `tfsdk:"max_version"`
	MaxAge         bigtableMaxAgeModel     `tfsdk:"max_age"`
	GcRules        types.String            `tfsdk:"gc_rules"`
}

type bigtableMaxVersionModel struct {
	Number types.Int64 `tfsdk:"number"`
}

type bigtableMaxAgeModel struct {
	Duration types.String `tfsdk:"duration"`
}

// Metadata returns the data source type name.
func (d *bigtableGarbageCollectionPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tables"
}

// Schema defines the schema for the data source.
func (d *bigtableGarbageCollectionPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
						"table": schema.StringAttribute{
							Computed: true,
						},
						"column_family": schema.StringAttribute{
							Computed: true,
						},
						"deletion_policy": schema.StringAttribute{
							Computed: true,
						},
						"gc_rules": schema.StringAttribute{
							Computed: true,
						},
						"max_version": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"number": schema.NumberAttribute{
									Computed: true,
								},
							},
						},
						"max_age": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"duration": schema.StringAttribute{
									Computed: true,
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
func (d *bigtableGarbageCollectionPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state bigtableGarbageCollectionPolicyModel

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	tableId := state.Table.ValueString()
	columnFamilyId := state.ColumFamily.ValueString()

	// Read garbage collection policy
	gcPolicy, err := d.client.GetGarbageCollectionPolicy(ctx, &pb.GetGarbageCollectionPolicyRequest{
		Parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
		ColumnFamilyId: columnFamilyId,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read DB Tables",
			err.Error(),
		)
		return
	}

	// Populate deletion policy
	switch gcPolicy.GetDeletionPolicy() {
	case pb.Table_ColumnFamily_GarbageCollectionPolicy_ABANDON:
		state.DeletionPolicy = types.StringValue("ABANDON")
	}

	// Populate rules
	if gcPolicy.GetGcRule() != nil {
		switch gcPolicy.GetGcRule().GetRule().(type) {
		case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_MaxNumVersions:
			state.MaxVersion = bigtableMaxVersionModel{
				Number: types.Int64Value(int64(gcPolicy.GetGcRule().GetMaxNumVersions())),
			}
		case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_MaxAge:
			if gcPolicy.GetGcRule().GetMaxAge() != nil {
				state.MaxAge = bigtableMaxAgeModel{
					Duration: types.StringValue(gcPolicy.GetGcRule().GetMaxAge().AsDuration().String()),
				}
			}
		case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Union_:
			// TODO: Implement Union
		case *pb.Table_ColumnFamily_GarbageCollectionPolicy_GcRule_Intersection_:
			// TODO: Implement Intersection
		}
	}

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *bigtableGarbageCollectionPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
