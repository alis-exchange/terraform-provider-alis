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
	_ datasource.DataSource              = &tableTTLPolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &tableTTLPolicyDataSource{}
)

// NewTableTTLPolicyDataSource is a helper function to simplify the provider implementation.
func NewTableTTLPolicyDataSource() datasource.DataSource {
	return &tableTTLPolicyDataSource{}
}

type tableTTLPolicyDataSource struct {
	service *services.SpannerService
}

type tableTTLPolicyDataModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Database types.String `tfsdk:"database"`
	Table    types.String `tfsdk:"table"`
	Column   types.String `tfsdk:"column"`
	TTL      types.Int64  `tfsdk:"ttl"`
}

// Metadata returns the data source type name.
func (d *tableTTLPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_table_ttl_policy"
}

// Schema defines the schema for the data source.
func (d *tableTTLPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
				MarkdownDescription: "The table whose row deletion (TTL) policy is read. A table has at most one policy.",
			},
			"column": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The `TIMESTAMP` column the policy is based on.",
			},
			"ttl": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Days after the timestamp in `column` at which a row becomes eligible for deletion.",
			},
		},
		MarkdownDescription: "Reads the row deletion (TTL) policy of a Cloud Spanner table. Reading a table without a policy, or a " +
			"table that does not exist, is an error. See https://cloud.google.com/spanner/docs/ttl/working-with-ttl",
	}
}

// Read fetches the policy; any failure, including NotFound, is an error.
func (d *tableTTLPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state tableTTLPolicyDataModel
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

	policy, err := d.service.GetSpannerTableRowDeletionPolicy(ctx, tableName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading TTL Policy",
			"Could not read TTL Policy on Table ("+tableName+"): "+utils.ErrDetail(err),
		)
		return
	}
	state.Column = types.StringValue(policy.Column)
	if policy.Duration != nil {
		state.TTL = types.Int64Value(policy.Duration.GetValue())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *tableTTLPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
