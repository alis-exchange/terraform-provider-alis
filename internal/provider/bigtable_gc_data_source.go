package provider

import (
	"context"
	"encoding/json"
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

type bigtableGarbageCollectionPoliciesDataSourceModel struct {
	GcPolicies []bigtableGarbageCollectionPolicyModel `tfsdk:"gc_policies"`
}

type bigtableGarbageCollectionPolicyModel struct {
	Project        types.String `tfsdk:"project"`
	InstanceName   types.String `tfsdk:"instance_name"`
	Table          types.String `tfsdk:"table"`
	ColumFamily    types.String `tfsdk:"column_family"`
	DeletionPolicy types.String `tfsdk:"deletion_policy"`
	GcRules        types.String `tfsdk:"gc_rules"`
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
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *bigtableGarbageCollectionPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state bigtableGarbageCollectionPoliciesDataSourceModel

	// Get project and instance name
	// TODO: Populate project and instance name
	project := ""
	instanceName := ""
	tableId := ""

	// Read garbage collection policy
	gcPoliciesRes, err := d.client.ListGarbageCollectionPolicies(ctx, &pb.ListGarbageCollectionPoliciesRequest{
		Parent: fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read DB Tables",
			err.Error(),
		)
		return
	}

	// Populate deletion policies
	for _, gcPolicy := range gcPoliciesRes.GetGcPolicies() {
		// TODO: Populate project, instance name, table, and column family
		policyModel := bigtableGarbageCollectionPolicyModel{}

		switch gcPolicy.GetDeletionPolicy() {
		case pb.Table_ColumnFamily_GarbageCollectionPolicy_ABANDON:
			policyModel.DeletionPolicy = types.StringValue("ABANDON")
		}

		// Populate rules
		if gcPolicy.GetGcRule() != nil {
			gcRuleMap, err := GcPolicyToGCRuleMap(gcPolicy.GetGcRule(), true)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to Parse GC Policy to GC Rule String",
					err.Error(),
				)
				return
			}

			gcRuleBytes, err := json.Marshal(gcRuleMap)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to Marshal GC Rule Map to JSON",
					err.Error(),
				)
				return
			}

			policyModel.GcRules = types.StringValue(string(gcRuleBytes))
		}

		state.GcPolicies = append(state.GcPolicies, policyModel)
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
