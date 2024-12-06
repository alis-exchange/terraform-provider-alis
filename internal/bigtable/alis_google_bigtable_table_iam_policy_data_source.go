package bigtable

import (
	"context"
	"fmt"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &tableIamPolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &tableIamPolicyDataSource{}
)

// NewIamPolicyDataSource is a helper function to simplify the data source implementation.
func NewIamPolicyDataSource() datasource.DataSource {
	return &tableIamPolicyDataSource{}
}

type tableIamPolicyDataSource struct {
	config *internal.ProviderConfig
}

type tableIamPolicyModel struct {
	Project  types.String `tfsdk:"project"`
	Instance types.String `tfsdk:"instance"`
	Table    types.String `tfsdk:"table"`
	Bindings types.List   `tfsdk:"bindings"`
}

type tableIamPolicyBindingModel struct {
	Role    types.String   `tfsdk:"role"`
	Members []types.String `tfsdk:"members"`
}

func (o tableIamPolicyBindingModel) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"role": types.StringType,
		"members": types.ListType{
			ElemType: types.StringType,
		},
	}
}

// Metadata returns the resource type name.
func (d *tableIamPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_bigtable_table_iam_policy"
}

// Schema defines the schema for the resource.
func (d *tableIamPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID.",
			},
			"instance": schema.StringAttribute{
				Required:    true,
				Description: "The Bigtable instance ID.",
			},
			"table": schema.StringAttribute{
				Required:    true,
				Description: "The Bigtable table ID.",
			},
			"bindings": schema.ListNestedAttribute{
				Computed: true,
				CustomType: types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: tableIamPolicyBindingModel{}.attrTypes(),
					},
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Computed: true,
						},
						"members": schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
						},
					},
				},
			},
		},
		DeprecationMessage: "This resource is deprecated. Please use the standard Google provider resource instead.",
	}
}

// Read resource information.
func (d *tableIamPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state tableIamPolicyModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from state
	project := state.Project.ValueString()
	instance := state.Instance.ValueString()
	table := state.Table.ValueString()

	policy, err := d.config.BigtableService.GetBigtableTableIamPolicy(ctx, fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instance, table), &iampb.GetPolicyOptions{
		RequestedPolicyVersion: internal.IamPolicyVersion,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to get Bigtable Table IAM Policy", err.Error())
		return
	}

	// Map response body to state
	if policy.GetBindings() != nil {
		policyBindings := make([]*tableIamPolicyBindingModel, 0)

		for _, binding := range policy.GetBindings() {
			bindingModel := &tableIamPolicyBindingModel{
				Role:    types.StringValue(binding.GetRole()),
				Members: make([]types.String, 0),
			}

			for _, member := range binding.GetMembers() {
				bindingModel.Members = append(bindingModel.Members, types.StringValue(member))
			}

			policyBindings = append(policyBindings, bindingModel)
		}

		generatedList, diagnostics := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: tableIamPolicyBindingModel{}.attrTypes(),
		}, policyBindings)
		diags.Append(diagnostics...)

		state.Bindings = generatedList
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the resource.
func (d *tableIamPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*internal.ProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *utils.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.config = config
}

func (d *tableIamPolicyDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		//datasourcevalidator.All(),
		//	datasourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
