package spanner

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &protoBundleDataSource{}
	_ datasource.DataSourceWithConfigure = &protoBundleDataSource{}
)

// NewProtoBundleDataSource is a helper function to simplify the provider implementation.
func NewProtoBundleDataSource() datasource.DataSource {
	return &protoBundleDataSource{}
}

type protoBundleDataSource struct {
	service *services.SpannerService
}

type protoBundleDataModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Database types.String `tfsdk:"database"`
	Packages types.Set    `tfsdk:"packages"`
	Types    types.Set    `tfsdk:"types"`
}

// Metadata returns the data source type name.
func (d *protoBundleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_proto_bundle"
}

// Schema defines the schema for the data source.
func (d *protoBundleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
				MarkdownDescription: "The Spanner database ID whose proto bundle is read.",
			},
			"packages": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Proto packages to look up. Only messages and enums directly in these packages are returned; nested packages are not included.",
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"types": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Fully qualified names of the messages and enums from `packages` that the bundle holds.",
			},
		},
		MarkdownDescription: "Reads the types a Cloud Spanner database's proto bundle holds for the given proto packages. " +
			"A bundle holding none of them (or no bundle at all) is an error, so this data source doubles as a guard " +
			"for tables with `PROTO` columns whose bundle is managed elsewhere.",
	}
}

// Read lists the owned types; any failure, including NotFound, is an error.
func (d *protoBundleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state protoBundleDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var packages []string
	resp.Diagnostics.Append(state.Packages.ElementsAs(ctx, &packages, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	database := names.DatabaseName{
		Project:  state.Project.ValueString(),
		Instance: state.Instance.ValueString(),
		Database: state.Database.ValueString(),
	}.String()

	owned, err := d.service.GetProtoBundleTypes(ctx, database, packages)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Proto Bundle",
			"Could not read the proto bundle of "+database+": "+utils.ErrDetail(err),
		)
		return
	}
	typesValue, diags := types.SetValueFrom(ctx, types.StringType, owned)
	resp.Diagnostics.Append(diags...)
	state.Types = typesValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Configure adds the provider configured client to the data source.
func (d *protoBundleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	d.service = service
}
