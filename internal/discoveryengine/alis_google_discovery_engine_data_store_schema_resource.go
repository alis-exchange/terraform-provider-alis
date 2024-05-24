package discoveryengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/discoveryengine/apiv1beta/discoveryenginepb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal"
	custom_types "terraform-provider-alis/internal/custom-types"
	"terraform-provider-alis/internal/discoveryengine/services"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &discoveryEngineDataStoreSchemaResource{}
	_ resource.ResourceWithConfigure   = &discoveryEngineDataStoreSchemaResource{}
	_ resource.ResourceWithImportState = &discoveryEngineDataStoreSchemaResource{}
)

// NewDiscoveryEngineDataSourceSchemaResource is a helper function to simplify the provider implementation.
func NewDiscoveryEngineDataSourceSchemaResource() resource.Resource {
	return &discoveryEngineDataStoreSchemaResource{}
}

type discoveryEngineDataStoreSchemaResource struct {
	config *internal.ProviderConfig
}

type discoveryEngineDataStoreSchemaModel struct {
	Project      types.String                 `tfsdk:"project"`
	Location     types.String                 `tfsdk:"location"`
	CollectionId types.String                 `tfsdk:"collection_id"`
	DataStoreId  types.String                 `tfsdk:"data_store_id"`
	SchemaId     types.String                 `tfsdk:"schema_id"`
	Schema       custom_types.JsonStringValue `tfsdk:"schema"`
}

func (o discoveryEngineDataStoreSchemaModel) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"project":       types.StringType,
		"location":      types.StringType,
		"collection_id": types.StringType,
		"data_store_id": types.StringType,
		"schema_id":     types.StringType,
		"schema":        custom_types.JsonStringType{},
	}
}

// Metadata returns the resource type name.
func (r *discoveryEngineDataStoreSchemaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_discovery_engine_data_store_schema"
}

// Schema defines the schema for the resource.
func (r *discoveryEngineDataStoreSchemaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Required:    true,
				Description: "The Datastore location",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"collection_id": schema.StringAttribute{
				Required:    true,
				Description: "The collection ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data_store_id": schema.StringAttribute{
				Required:    true,
				Description: "The unique id of the data store.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema_id": schema.StringAttribute{
				Required:    true,
				Description: "The unique id of the schema.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				CustomType: custom_types.JsonStringType{},
				Required:   true,
				Description: "JSON stringified schema definition using the [JSON Schema](https://json-schema.org/) format. See https://cloud.google.com/generative-ai-app-builder/docs/provide-schema\n" +
					"	- `keyPropertyMapping` A field that maps predefined keywords to critical fields in your documents,\n" +
					"	helping to clarify their semantic meaning. Values include title, description, uri, and category.\n" +
					"	Note that your field name doesn't need to match the keyPropertyValues value.\n" +
					"	For example, for a field that you named my_title, you can include a keyPropertyValues field with a value of title.\n" +
					"	For Vertex AI Search: fields marked with keyPropertyMapping are by default indexable and searchable, but not retrievable, completable, or dynamicFacetable.\n" +
					"	This means that you don't need to include the indexable or searchable fields with a keyPropertyValues field to get the expected default behavior.\n" +
					"	- `type` The type of the field. This is a string value that is `integer`, `datetime`, `geolocation` or one of the primative types (`boolean`, `object`, `array`, `number`, or `string`).\n" +
					"	- `retrievable` Indicates whether this field can be returned in a search response. This can be set for fields of type number, string, boolean, integer, datetime, and geolocation.\n" +
					"	A maximum of 50 fields can be set as retrievable. User-defined fields and keyPropertyValues fields are not retrievable by default. To make a field retrievable, include `\"retrievable\": true` with the field.\n" +
					"	- `indexable` Indicates whether this field can be filtered, faceted, boosted, or sorted. This can be set for fields of type number, string, boolean, integer, datetime, and geolocation.\n" +
					"	A maximum of 50 fields can be set as indexable. User-defined fields are not indexable by default, except for fields containing the keyPropertyMapping field.\n" +
					"	To make a field indexable, include `\"indexable\": true` with the field.\n" +
					"	- `dynamicFacetable` Indicates that the field can be used as a dynamic facet. This can be set for fields of type number, string, boolean, and integer. To make a field dynamically facetable, include `\"dynamicFacetable\": true` with the field.\n" +
					"	- `searchable` Indicates whether this field can be reverse indexed to match unstructured text queries. This can only be set for fields of type string. A maximum of 50 fields can be set as searchable.\n" +
					"	User-defined fields are not searchable by default, except for fields containing the keyPropertyMapping field. To make a field searchable, include `\"searchable\": true` with the field.\n" +
					"	- `completable` Indicates whether this field can be returned as an autocomplete suggestion. This can only be set for fields of type string. To make a field completable, include \"completable\": true with the field.",
			},
		},
		Description: "A Google Discovery Engine DataStore Schema resource.\n" +
			"This resource provisions and manages schemas for a Google Discovery Engine DataStore.",
	}
}

// Create a new resource.
func (r *discoveryEngineDataStoreSchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan discoveryEngineDataStoreSchemaModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	location := plan.Location.ValueString()
	collectionId := plan.CollectionId.ValueString()
	dataStoreId := plan.DataStoreId.ValueString()
	schemaId := plan.SchemaId.ValueString()
	jsonSchemaStr := plan.Schema.ValueString()

	// Validate json schema
	jsonSchema := map[string]interface{}{}
	err := json.Unmarshal([]byte(jsonSchemaStr), &jsonSchema)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Unmarshalling JSON Schema",
			"Unable to Unmarshal JSON Schema: "+err.Error(),
		)
		return
	}
	err = services.ValidateJsonSchema(jsonSchema, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Validating JSON Schema",
			"Invalid JSON Schema: "+err.Error(),
		)
		return
	}

	// Create schema
	createdSchema, err := r.config.DiscoveryEngineService.CreateDiscoveryEngineDatastoreSchema(ctx,
		fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s", project, location, collectionId, dataStoreId),
		schemaId,
		&discoveryenginepb.Schema{
			Name: "",
			Schema: &discoveryenginepb.Schema_JsonSchema{
				JsonSchema: jsonSchemaStr,
			},
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Schema",
			"Could not create Schema ("+schemaId+") in datastore ("+dataStoreId+"): "+err.Error(),
		)
		return
	}

	// Refresh state with created schema
	switch createdSchema.GetSchema().(type) {
	case *discoveryenginepb.Schema_JsonSchema:
		plan.Schema = custom_types.NewJsonStringValue(createdSchema.GetJsonSchema())
	case *discoveryenginepb.Schema_StructSchema:
		jsonSchemaMap := createdSchema.GetStructSchema().AsMap()
		jsonSchemaBytes, err := json.Marshal(jsonSchemaMap)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Marshalling Schema Map to JSON",
				"Unable to Marshal Schema Map to JSON: "+err.Error(),
			)
			return
		}

		plan.Schema = custom_types.NewJsonStringValue(string(jsonSchemaBytes))
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *discoveryEngineDataStoreSchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state discoveryEngineDataStoreSchemaModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	location := state.Location.ValueString()
	collectionId := state.CollectionId.ValueString()
	dataStoreId := state.DataStoreId.ValueString()
	schemaId := state.SchemaId.ValueString()

	// Get schema from API
	sch, err := r.config.DiscoveryEngineService.GetDiscoveryEngineDatastoreSchema(ctx, fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s/schemas/%s", project, location, collectionId, dataStoreId, schemaId))
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Schema",
			"Could not read Schema ("+state.SchemaId.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Set refreshed state with schema
	switch sch.GetSchema().(type) {
	case *discoveryenginepb.Schema_JsonSchema:
		state.Schema = custom_types.NewJsonStringValue(sch.GetJsonSchema())
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

		state.Schema = custom_types.NewJsonStringValue(string(jsonSchemaBytes))
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *discoveryEngineDataStoreSchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan discoveryEngineDataStoreSchemaModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	location := plan.Location.ValueString()
	collectionId := plan.CollectionId.ValueString()
	dataStoreId := plan.DataStoreId.ValueString()
	schemaId := plan.SchemaId.ValueString()
	jsonSchemaStr := plan.Schema.ValueString()

	// Validate json schema
	jsonSchema := map[string]interface{}{}
	err := json.Unmarshal([]byte(jsonSchemaStr), &jsonSchema)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Unmarshalling JSON Schema",
			"Unable to Unmarshal JSON Schema: "+err.Error(),
		)
		return
	}
	err = services.ValidateJsonSchema(jsonSchema, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Validating JSON Schema",
			"Invalid JSON Schema: "+err.Error(),
		)
		return
	}

	// Update existing schema
	updatedSchema, err := r.config.DiscoveryEngineService.UpdateDiscoveryEngineDatastoreSchema(ctx, &discoveryenginepb.Schema{
		Name: fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s/schemas/%s", project, location, collectionId, dataStoreId, schemaId),
		Schema: &discoveryenginepb.Schema_JsonSchema{
			JsonSchema: jsonSchemaStr,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Schema",
			"Could not update Schema ("+schemaId+"): "+err.Error(),
		)
		return
	}

	// Refresh state with updated schema
	switch updatedSchema.GetSchema().(type) {
	case *discoveryenginepb.Schema_JsonSchema:
		plan.Schema = custom_types.NewJsonStringValue(updatedSchema.GetJsonSchema())
	case *discoveryenginepb.Schema_StructSchema:
		jsonSchemaMap := updatedSchema.GetStructSchema().AsMap()
		jsonSchemaBytes, err := json.Marshal(jsonSchemaMap)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Marshalling Schema Map to JSON",
				"Unable to Marshal Schema Map to JSON: "+err.Error(),
			)
			return
		}

		plan.Schema = custom_types.NewJsonStringValue(string(jsonSchemaBytes))
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *discoveryEngineDataStoreSchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state discoveryEngineDataStoreSchemaModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	location := state.Location.ValueString()
	collectionId := state.CollectionId.ValueString()
	dataStoreId := state.DataStoreId.ValueString()
	schemaId := state.SchemaId.ValueString()

	// Delete existing table
	err := r.config.DiscoveryEngineService.DeleteDiscoveryEngineDatastoreSchema(ctx, fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s/schemas/%s", project, location, collectionId, dataStoreId, schemaId))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Schema",
			"Could not delete Schema ("+schemaId+"): "+err.Error(),
		)
		return
	}
}

func (r *discoveryEngineDataStoreSchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and table id
	// projects/{project}/locations/{location}/collections/{collection}/dataStores/{data_store}/schemas/{schema}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 10 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format `projects/{project}/locations/{location}/collections/{collection}/dataStores/{data_store}/schemas/{schema}`",
		)
		return
	}
	project := importIDParts[1]
	location := importIDParts[3]
	collectionId := importIDParts[5]
	dataStoreId := importIDParts[7]
	schemaId := importIDParts[9]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("location"), location)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("collection_id"), collectionId)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("data_store_id"), dataStoreId)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("schema_id"), schemaId)...)
}

// Configure adds the provider configured client to the resource.
func (r *discoveryEngineDataStoreSchemaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*internal.ProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
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

	r.config = config
}

func (r *discoveryEngineDataStoreSchemaResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
