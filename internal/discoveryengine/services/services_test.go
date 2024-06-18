package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	discoveryenginepb "cloud.google.com/go/discoveryengine/apiv1/discoveryenginepb"
	googleoauth "golang.org/x/oauth2/google"
)

var (
	// TestProject is the project used for testing.
	TestProject string
	// TestDatastore is the datastore used for testing.
	TestDatastore string
	service       *DiscoveryEngineService
)

func init() {
	TestProject = os.Getenv("ALIS_OS_PROJECT")
	TestDatastore = os.Getenv("ALIS_OS_DISCOVERY_ENGINE_DATASTORE")

	if TestProject == "" {
		log.Fatalf("ALIS_OS_PROJECT must be set for integration tests")
	}

	if TestDatastore == "" {
		log.Fatalf("ALIS_OS_DISCOVERY_ENGINE_DATASTORE must be set for integration tests")
	}

	service = NewDiscoveryEngineService(nil)
}

func TestDiscoveryEngineService_GetDiscoveryEngineDatastoreSchema(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *discoveryenginepb.Schema
		wantErr bool
	}{
		{
			name: "Test GetDiscoveryEngineDatastoreSchema",
			args: args{
				ctx: context.Background(),
				// `projects/{project}/ locations/{location}/ collections/{collection}/ dataStores/{data_store}/ schemas/{schema}`.
				name: fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s/schemas/%s", TestProject, "global", "default_collection", TestDatastore, "default_schema"),
			},
			want:    &discoveryenginepb.Schema{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetDiscoveryEngineDatastoreSchema(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDiscoveryEngineDatastoreSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDiscoveryEngineDatastoreSchema() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoveryEngineService_CreateDiscoveryEngineDatastoreSchema(t *testing.T) {

	// {"type":"object","$schema":"https://json-schema.org/draft/2020-12/schema","properties":{"memo":{"dynamicFacetable":true,"type":"string","searchable":true,"indexable":true,"retrievable":true}}}
	schema := map[string]interface{}{
		"type":    "object",
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"properties": map[string]interface{}{
			"memo": map[string]interface{}{
				"dynamicFacetable": true,
				"type":             "string",
				"searchable":       true,
				"indexable":        true,
				"retrievable":      true,
			},
		},
	}

	schemaJsonBytes, err := json.Marshal(schema)
	if err != nil {
		t.Errorf("Error marshalling schema: %v", err)
		return
	}

	schemaJsonStr := string(schemaJsonBytes)

	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx      context.Context
		parent   string
		schemaId string
		schema   *discoveryenginepb.Schema
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *discoveryenginepb.Schema
		wantErr bool
	}{
		{
			name: "Test CreateDiscoveryEngineDatastoreSchema",
			args: args{
				ctx:      context.Background(),
				parent:   fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s", TestProject, "global", "default_collection", TestDatastore),
				schemaId: "test_schema",
				schema: &discoveryenginepb.Schema{
					Schema: &discoveryenginepb.Schema_JsonSchema{
						JsonSchema: schemaJsonStr,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateDiscoveryEngineDatastoreSchema(tt.args.ctx, tt.args.parent, tt.args.schemaId, tt.args.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDiscoveryEngineDatastoreSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateDiscoveryEngineDatastoreSchema() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoveryEngineService_UpdateDiscoveryEngineDatastoreSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type":    "object",
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"properties": map[string]interface{}{
			"memo": map[string]interface{}{
				"dynamicFacetable": true,
				"type":             "string",
				"searchable":       true,
				"indexable":        true,
				"retrievable":      true,
			},
		},
	}

	schemaJsonBytes, err := json.Marshal(schema)
	if err != nil {
		t.Errorf("Error marshalling schema: %v", err)
		return
	}

	schemaJsonStr := string(schemaJsonBytes)

	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		schema *discoveryenginepb.Schema
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *discoveryenginepb.Schema
		wantErr bool
	}{
		{
			name: "Test UpdateDiscoveryEngineDatastoreSchema",
			args: args{
				ctx: context.Background(),
				schema: &discoveryenginepb.Schema{
					Name: fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s/schemas/%s", TestProject, "global", "default_collection", TestDatastore, "default_schema"),
					Schema: &discoveryenginepb.Schema_JsonSchema{
						JsonSchema: schemaJsonStr,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateDiscoveryEngineDatastoreSchema(tt.args.ctx, tt.args.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateDiscoveryEngineDatastoreSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateDiscoveryEngineDatastoreSchema() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoveryEngineService_ListDiscoveryEngineDatastoreSchemas(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx       context.Context
		parent    string
		pageSize  int32
		pageToken string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*discoveryenginepb.Schema
		want1   string
		wantErr bool
	}{
		{
			name: "Test ListDiscoveryEngineDatastoreSchemas",
			args: args{
				ctx:       context.Background(),
				parent:    fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s", TestProject, "global", "default_collection", TestDatastore),
				pageSize:  100,
				pageToken: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := service.ListDiscoveryEngineDatastoreSchemas(tt.args.ctx, tt.args.parent, tt.args.pageSize, tt.args.pageToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListDiscoveryEngineDatastoreSchemas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListDiscoveryEngineDatastoreSchemas() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ListDiscoveryEngineDatastoreSchemas() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDiscoveryEngineService_DeleteDiscoveryEngineDatastoreSchema(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Test DeleteDiscoveryEngineDatastoreSchema",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/locations/%s/collections/%s/dataStores/%s/schemas/%s", TestProject, "global", "default_collection", TestDatastore, "default_schema"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := service.DeleteDiscoveryEngineDatastoreSchema(tt.args.ctx, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteDiscoveryEngineDatastoreSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
