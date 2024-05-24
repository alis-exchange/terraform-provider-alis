package services

import (
	"context"
	"errors"

	discoveryengine "cloud.google.com/go/discoveryengine/apiv1beta"
	discoveryenginepb "cloud.google.com/go/discoveryengine/apiv1beta/discoveryenginepb"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal/utils"
)

type DiscoveryEngineService struct {
	GoogleCredentials *googleoauth.Credentials
}

func NewDiscoveryEngineService(creds *googleoauth.Credentials) *DiscoveryEngineService {
	return &DiscoveryEngineService{
		GoogleCredentials: creds,
	}
}

func (s *DiscoveryEngineService) GetDiscoveryEngineDatastoreSchema(ctx context.Context, name string) (*discoveryenginepb.Schema, error) {
	// Validate arguments
	if valid := utils.ValidateArgument(name, utils.DiscoveryEngineDatastoreSchemaNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.DiscoveryEngineDatastoreSchemaNameRegex)
	}

	// If Google credentials are provided, use them instead of Application Default Credentials
	opts := make([]option.ClientOption, 0)
	if s.GoogleCredentials != nil {
		opts = append(opts, option.WithCredentials(s.GoogleCredentials))
	}
	client, err := discoveryengine.NewSchemaClient(ctx, opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating Discovery Engine Schema client: %v", err)
	}
	defer client.Close()

	schema, err := client.GetSchema(ctx, &discoveryenginepb.GetSchemaRequest{
		Name: name,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "Schema (%s) not found: %v", name, err)
		}

		return nil, status.Errorf(codes.Internal, "Error getting schema (%s): %v", name, err)
	}

	return schema, nil
}

func (s *DiscoveryEngineService) CreateDiscoveryEngineDatastoreSchema(ctx context.Context, parent string, schemaId string, schema *discoveryenginepb.Schema) (*discoveryenginepb.Schema, error) {
	// Validate arguments
	if valid := utils.ValidateArgument(parent, utils.DiscoveryEngineDatastoreNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.DiscoveryEngineDatastoreNameRegex)
	}
	if valid := utils.ValidateArgument(schemaId, utils.DiscoveryEngineDatastoreSchemaIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument schemaId (%s), must match `%s`", schemaId, utils.DiscoveryEngineDatastoreSchemaIdRegex)
	}

	// If Google credentials are provided, use them instead of Application Default Credentials
	opts := make([]option.ClientOption, 0)
	if s.GoogleCredentials != nil {
		opts = append(opts, option.WithCredentials(s.GoogleCredentials))
	}
	client, err := discoveryengine.NewSchemaClient(ctx, opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating Discovery Engine Schema client: %v", err)
	}
	defer client.Close()

	createSchemaOperation, err := client.CreateSchema(ctx, &discoveryenginepb.CreateSchemaRequest{
		// projects/{project}/ locations/{location}/ collections/{collection}/ dataStores/{data_store}
		Parent:   parent,
		Schema:   schema,
		SchemaId: schemaId,
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return nil, status.Errorf(codes.AlreadyExists, "Schema (%s) already exists: %v", schemaId, err)
		}

		return nil, status.Errorf(codes.Internal, "Error creating schema: %v", err)
	}

	createdSchema, err := createSchemaOperation.Wait(ctx)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return nil, status.Errorf(codes.AlreadyExists, "Schema (%s) already exists: %v", schemaId, err)
		}

		return nil, status.Errorf(codes.Internal, "Error creating schema: %v", err)
	}

	return createdSchema, nil
}

func (s *DiscoveryEngineService) UpdateDiscoveryEngineDatastoreSchema(ctx context.Context, schema *discoveryenginepb.Schema) (*discoveryenginepb.Schema, error) {
	// Validate arguments
	if valid := utils.ValidateArgument(schema.GetName(), utils.DiscoveryEngineDatastoreSchemaNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", schema.GetName(), utils.DiscoveryEngineDatastoreSchemaNameRegex)
	}

	// If Google credentials are provided, use them instead of Application Default Credentials
	opts := make([]option.ClientOption, 0)
	if s.GoogleCredentials != nil {
		opts = append(opts, option.WithCredentials(s.GoogleCredentials))
	}
	client, err := discoveryengine.NewSchemaClient(ctx, opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating Discovery Engine Schema client: %v", err)
	}
	defer client.Close()

	updateSchemaOperation, err := client.UpdateSchema(ctx, &discoveryenginepb.UpdateSchemaRequest{
		Schema: schema,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error updating schema: %v", err)
	}

	updatedSchema, err := updateSchemaOperation.Wait(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error updating schema: %v", err)
	}

	return updatedSchema, nil
}

func (s *DiscoveryEngineService) ListDiscoveryEngineDatastoreSchemas(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*discoveryenginepb.Schema, string, error) {
	// Validate arguments
	if valid := utils.ValidateArgument(parent, utils.DiscoveryEngineDatastoreNameRegex); !valid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.DiscoveryEngineDatastoreNameRegex)
	}

	// If Google credentials are provided, use them instead of Application Default Credentials
	opts := make([]option.ClientOption, 0)
	if s.GoogleCredentials != nil {
		opts = append(opts, option.WithCredentials(s.GoogleCredentials))
	}
	client, err := discoveryengine.NewSchemaClient(ctx, opts...)
	if err != nil {
		return nil, "", status.Errorf(codes.Internal, "Error creating Discovery Engine Schema client: %v", err)
	}
	defer client.Close()

	it := client.ListSchemas(ctx, &discoveryenginepb.ListSchemasRequest{
		Parent:    parent,
		PageSize:  pageSize,
		PageToken: pageToken,
	})

	res := make([]*discoveryenginepb.Schema, 0)
	var nextPageToken string
	for {
		schema, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "Error listing schemas: %v", err)
		}
		res = append(res, schema)
	}

	nextPageToken = it.PageInfo().Token

	return res, nextPageToken, nil
}

func (s *DiscoveryEngineService) DeleteDiscoveryEngineDatastoreSchema(ctx context.Context, name string) error {
	// Validate arguments
	if valid := utils.ValidateArgument(name, utils.DiscoveryEngineDatastoreSchemaNameRegex); !valid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.DiscoveryEngineDatastoreSchemaNameRegex)
	}

	// If Google credentials are provided, use them instead of Application Default Credentials
	opts := make([]option.ClientOption, 0)
	if s.GoogleCredentials != nil {
		opts = append(opts, option.WithCredentials(s.GoogleCredentials))
	}
	client, err := discoveryengine.NewSchemaClient(ctx, opts...)
	if err != nil {
		return status.Errorf(codes.Internal, "Error creating Discovery Engine Schema client: %v", err)
	}
	defer client.Close()

	deleteSchemaOperation, err := client.DeleteSchema(ctx, &discoveryenginepb.DeleteSchemaRequest{
		Name: name,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return status.Errorf(codes.NotFound, "Schema (%s) not found: %v", name, err)
		}

		return status.Errorf(codes.Internal, "Error deleting schema: %v", err)
	}

	err = deleteSchemaOperation.Wait(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Error deleting schema: %v", err)
	}

	return nil
}
