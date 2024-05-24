package discoveryengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/discoveryengine/apiv1beta/discoveryenginepb"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-alis/internal"
	custom_types "terraform-provider-alis/internal/custom-types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &discoveryEngineDataStoreSchemasDataSource{}
	_ datasource.DataSourceWithConfigure = &discoveryEngineDataStoreSchemasDataSource{}
)

// NewDiscoveryEngineDataStoreSchemasDataSource is a helper function to simplify the data source implementation.
func NewDiscoveryEngineDataStoreSchemasDataSource() datasource.DataSource {
	return &discoveryEngineDataStoreSchemasDataSource{}
}

type discoveryEngineDataStoreSchemasDataSource struct {
	config *internal.ProviderConfig
}

type discoveryEngineDataStoreSchemasModel struct {
	Project      types.String `tfsdk:"project"`
	Location     types.String `tfsdk:"location"`
	CollectionId types.String `tfsdk:"collection_id"`
	DataStoreId  types.String `tfsdk:"data_store_id"`
	Schemas      types.List   `tfsdk:"schemas"`
}

// Metadata returns the resource type name.
func (d *discoveryEngineDataStoreSchemasDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_discovery_engine_data_store_schemas"
}

// Schema defines the schema for the resource.
func (d *discoveryEngineDataStoreSchemasDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID.",
			},
			"location": schema.StringAttribute{
				Required:    true,
				Description: "The Datastore location",
			},
			"collection_id": schema.StringAttribute{
				Required:    true,
				Description: "The collection ID.",
			},
			"data_store_id": schema.StringAttribute{
				Required:    true,
				Description: "The unique id of the data store.",
			},
			"schemas": schema.ListNestedAttribute{
				Computed: true,
				CustomType: types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: discoveryEngineDataStoreSchemaModel{}.attrTypes(),
					},
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"project": schema.StringAttribute{
							Computed:    true,
							Description: "The Google Cloud project ID.",
						},
						"location": schema.StringAttribute{
							Computed:    true,
							Description: "The Datastore location",
						},
						"collection_id": schema.StringAttribute{
							Computed:    true,
							Description: "The collection ID.",
						},
						"data_store_id": schema.StringAttribute{
							Computed:    true,
							Description: "The unique id of the data store.",
						},
						"schema_id": schema.StringAttribute{
							Computed:    true,
							Description: "The unique id of the schema.",
						},
						"schema": schema.StringAttribute{
							CustomType:  custom_types.JsonStringType{},
							Computed:    true,
							Description: "JSON stringified schema definition.",
						},
					},
				},
				Description: "List of schemas.",
			},
		},
	}
}

// Read resource information.
func (d *discoveryEngineDataStoreSchemasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state discoveryEngineDataStoreSchemasModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from state
	project := state.Project.ValueString()
	location := state.Location.ValueString()
	collectionId := state.CollectionId.ValueString()
	dataStoreId := state.DataStoreId.ValueString()

	var schemas []*discoveryenginepb.Schema
	var nextPageToken string
	for {
		// List schemas from API
		schemasRes, pageToken, err := d.config.DiscoveryEngineService.ListDiscoveryEngineDatastoreSchemas(ctx, fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s", project, location, collectionId, dataStoreId), 100, nextPageToken)
		if err != nil {
			resp.Diagnostics.AddError("Failed to list Datastore Schemas", err.Error())
			return
		}

		// Append schemas
		schemas = append(schemas, schemasRes...)

		// Check if we've reached the end of the results,
		// This is indicated by an empty page token
		nextPageToken = pageToken
		if nextPageToken == "" {
			break
		}
	}

	// Define a slice to hold our plan's schemas
	planSchemas := make([]*discoveryEngineDataStoreSchemaModel, 0)
	for _, sch := range schemas {
		// Split the schema name to get project, location, collection, data store and schema id
		schemaNameParts := strings.Split(sch.GetName(), "/")
		schemaProject := schemaNameParts[1]
		schemaLocation := schemaNameParts[3]
		schemaCollectionId := schemaNameParts[5]
		schemaDataStoreId := schemaNameParts[7]
		schemaSchemaId := schemaNameParts[9]

		schemaModel := &discoveryEngineDataStoreSchemaModel{
			Project:      types.StringValue(schemaProject),
			Location:     types.StringValue(schemaLocation),
			CollectionId: types.StringValue(schemaCollectionId),
			DataStoreId:  types.StringValue(schemaDataStoreId),
			SchemaId:     types.StringValue(schemaSchemaId),
		}

		switch sch.GetSchema().(type) {
		case *discoveryenginepb.Schema_JsonSchema:
			schemaModel.Schema = custom_types.NewJsonStringValue(sch.GetJsonSchema())
		case *discoveryenginepb.Schema_StructSchema:
			jsonSchemaMap := sch.GetStructSchema().AsMap()
			jsonSchemaBytes, err := json.Marshal(jsonSchemaMap)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Marshalling Schema Map to JSON",
					"Unable to Marshal Schema Map to JSON: "+err.Error(),
				)
				return
			}

			schemaModel.Schema = custom_types.NewJsonStringValue(string(jsonSchemaBytes))
		}

		// Append the schema to the plan
		planSchemas = append(planSchemas, schemaModel)
	}

	generatedList, diagnostics := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: discoveryEngineDataStoreSchemaModel{}.attrTypes(),
	}, planSchemas)
	diags.Append(diagnostics...)

	state.Schemas = generatedList

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the resource.
func (d *discoveryEngineDataStoreSchemasDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	if config.DiscoveryEngineService == nil {
		resp.Diagnostics.AddError(
			"Discovery Engine Service Not Configured",
			"Discovery Engine Service is not configured. Please report this issue to the provider developers.",
		)

		return
	}

	d.config = config
}

func (d *discoveryEngineDataStoreSchemasDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		//datasourcevalidator.All(),
		//	datasourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
