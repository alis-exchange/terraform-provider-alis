package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	spanner "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	spannergorm "github.com/googleapis/go-gorm-spanner"
	_ "github.com/googleapis/go-sql-spanner"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	customloggers "terraform-provider-alis/internal/spanner/logger"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"
)

var tfLogger = customloggers.New(
	log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
	logger.Config{
		SlowThreshold:             200 * time.Millisecond, // Slow SQL threshold
		LogLevel:                  logger.Info,            // Log level
		IgnoreRecordNotFoundError: false,                  // Ignore ErrRecordNotFound error for logger
		ParameterizedQueries:      true,                   // Don't include params in the SQL log
		Colorful:                  true,                   // Disable color
	},
)

const (
	DatabaseDialect_GoogleStandardSQL = "GOOGLE_STANDARD_SQL"
	DatabaseDialect_PostgreSQL        = "POSTGRESQL"

	DatabaseState_Creating        = "CREATING"
	DatabaseState_Ready           = "READY"
	DatabaseState_ReadyOptimizing = "READY_OPTIMIZING"

	DatabaseEncryptionType_CustomerManaged         = "CUSTOMER_MANAGED_ENCRYPTION"
	DatabaseEncryptionType_GoogleDefaultEncryption = "GOOGLE_DEFAULT_ENCRYPTION"
)

type SpannerService struct {
	GoogleCredentials *googleoauth.Credentials
}

func NewSpannerService(creds *googleoauth.Credentials) *SpannerService {
	return &SpannerService{
		GoogleCredentials: creds,
	}
}

func (s *SpannerService) CreateDatabaseRole(ctx context.Context, parent string, roleId string) (*databasepb.DatabaseRole, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure role is provided
	if roleId == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument roleId, field is required but not provided")
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: parent,
	})
	if err != nil {
		return nil, err
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("CREATE ROLE %s", roleId))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("CREATE ROLE %s", roleId))
	}
	updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   database.GetName(),
		Statements: ddlStatements,
	})
	if err != nil {
		return nil, err
	}

	// Wait for LRO to complete
	err = updateDatabaseDdlOperation.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return &databasepb.DatabaseRole{
		Name: fmt.Sprintf("%s/databaseRoles/%s", parent, roleId),
	}, nil
}

func (s *SpannerService) GetDatabaseRole(ctx context.Context, name string) (*databasepb.DatabaseRole, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseRoleNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseRoleNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Decompose name to get project, instance, database
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]

	var role *databasepb.DatabaseRole
	it := client.ListDatabaseRoles(ctx, &databasepb.ListDatabaseRolesRequest{
		Parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	for {
		r, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		if r.GetName() == name {
			role = r
			break
		}
	}

	if role == nil {
		return nil, status.Errorf(codes.NotFound, "Database role (%s) not found", name)
	}

	return role, nil
}

func (s *SpannerService) ListDatabaseRoles(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*databasepb.DatabaseRole, string, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	var res []*databasepb.DatabaseRole
	var nextPageToken string

	it := client.ListDatabaseRoles(ctx, &databasepb.ListDatabaseRolesRequest{
		Parent:    parent,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	for {
		r, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		res = append(res, r)

		// Check if page size is reached
		if pageSize > 0 && len(res) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
	}

	return res, nextPageToken, nil
}

func (s *SpannerService) DeleteDatabaseRole(ctx context.Context, name string) error {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseRoleNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseRoleNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Decompose name to get project, instance, database
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	roleId := nameParts[7]

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	if err != nil {
		return err
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("DROP ROLE %s", roleId))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("DROP ROLE %s", roleId))
	}
	updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   database.GetName(),
		Statements: ddlStatements,
	})
	if err != nil {
		return err
	}

	// Wait for LRO to complete
	err = updateDatabaseDdlOperation.Wait(ctx)
	if err != nil {
		return err
	}

	return nil
}

// CreateSpannerTable creates a new Spanner table.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the database that will serve the new table.
//   - tableId: string - Required. The ID of the table to create.
//   - table: *SpannerTable - Required. The table to create.
//
// Returns: *SpannerTable
func (s *SpannerService) CreateSpannerTable(ctx context.Context, parent string, tableId string, table *schema.SpannerTable) (*schema.SpannerTable, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}
	// Validate table id
	googleSqlTableValid := utils.ValidateArgument(tableId, utils.SpannerGoogleSqlTableIdRegex)
	postgresSqlTableValid := utils.ValidateArgument(tableId, utils.SpannerPostgresSqlTableIdRegex)
	if !googleSqlTableValid && !postgresSqlTableValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table_id (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", tableId, utils.SpannerGoogleSqlTableIdRegex, utils.SpannerPostgresSqlTableIdRegex)
	}
	// Ensure table is provided
	if table == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table, field is required but not provided")
	}
	// Ensure schema is provided
	if table.GetSchema() == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema, field is required but not provided")
	}
	// Ensure columns are provided and not empty
	if table.GetSchema() == nil || table.GetSchema().GetColumns() == nil || len(table.GetSchema().GetColumns()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema.columns, field is required but not provided")
	}
	// Validate columns
	for i, column := range table.GetSchema().GetColumns() {
		// Validate column name
		if valid := utils.ValidateArgument(column.GetName(), utils.SpannerGoogleSqlColumnIdRegex); !valid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].name (%s), must match `%s`", i, column.GetName(), utils.SpannerGoogleSqlColumnIdRegex)
		}

		// Ensure a type is provided
		if column.Type == "" {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].type, field is required but not provided", i)
		}

		// If column type is PROTO ensure proto file descriptor set is provided
		if column.Type == schema.SpannerTableDataTypeProto.String() {
			if column.GetProtoFileDescriptorSet() == nil {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set, field is required but not provided", i)
			}

			if column.GetProtoFileDescriptorSet().GetProtoPackage() == nil {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set.proto_package, field is required but not provided", i)
			}

			//if column.ProtoFileDescriptorSet.FileDescriptorSetPath == nil {
			//	return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set.file_descriptor_set_path, field is required but not provided", i)
			//}

			//if column.ProtoFileDescriptorSet.FileDescriptorSetPathSource == ProtoFileDescriptorSetSourceUNSPECIFIED {
			//	return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set.file_descriptor_set_path_source, field is required but not not provided", i)
			//}
		}
	}

	// Set table name
	table.Name = fmt.Sprintf("%s/tables/%s", parent, tableId)

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]

	// Check if we have any PROTO columns and create the necessary proto bundles
	for _, column := range table.GetSchema().GetColumns() {
		if column.GetType() != schema.SpannerTableDataTypeProto.String() {
			continue
		}

		// If file descriptor set path is not provided, skip
		// In such cases, we assume the user has already created the proto bundle(s)
		if column.GetProtoFileDescriptorSet().GetFileDescriptorSetPath() == nil || column.GetProtoFileDescriptorSet().GetFileDescriptorSetPath().GetValue() == "" {
			continue
		}

		fdsBytes, err := fetchDescriptorSet(ctx, column.GetProtoFileDescriptorSet())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Error fetching proto file descriptor set: %v", err)
		}

		// Unmarshal the proto file descriptor set
		fds := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(fdsBytes, fds); err != nil {
			return nil, status.Errorf(codes.Internal, "Error unmarshalling proto file descriptor set: %v", err)
		}

		// Update the column proto file descriptor set with the fds
		column.GetProtoFileDescriptorSet().SetFileDescriptorSet(fds)

		// Create proto bundle
		if err := CreateProtoBundle(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), column.GetProtoFileDescriptorSet().GetProtoPackage().GetValue(), fdsBytes); err != nil {
			return nil, status.Errorf(codes.Internal, "Error creating proto bundle: %v", err)
		}

	}

	// Create table
	_, err := utils.Retry(3, 5*time.Second, func() (interface{}, error) {
		_, err := table.Create(ctx)
		if err != nil {
			if status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Unavailable {
				return nil, err
			}

			return nil, utils.NonRetryableError(err)
		}

		return nil, nil
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition && strings.Contains(err.Error(), fmt.Sprintf("Duplicate name in schema: %s", tableId)) {
			return nil, status.Errorf(codes.AlreadyExists, "Table (%s) already exists", table.GetName())
		}

		return nil, err
	}

	// Get created table
	updatedTable, err := s.GetSpannerTable(ctx, table.GetName())
	if err != nil {
		return nil, err
	}

	return updatedTable, nil
}

// GetSpannerTable gets a Spanner table.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the table to get.
//
// Returns: *SpannerTable
func (s *SpannerService) GetSpannerTable(ctx context.Context, name string) (*schema.SpannerTable, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	table, err := (&schema.SpannerTable{}).Get(ctx, name)
	if err != nil {
		if (errors.Is(err, schema.ErrTableNotFound{})) {
			return nil, status.Errorf(codes.NotFound, "Table (%s) not found", name)
		}

		return nil, err
	}

	return table, nil
}

// UpdateSpannerTable updates a Spanner table.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - table: *SpannerTable - Required. The table to update.
//   - allowMissing: bool - If true and the table does not exist, a new table will be created. Default is false.
//
// Returns: *SpannerTable
func (s *SpannerService) UpdateSpannerTable(ctx context.Context, table *schema.SpannerTable, updateMask *fieldmaskpb.FieldMask, allowMissing bool) (*schema.SpannerTable, error) {
	// Validate arguments
	// Ensure table is provided
	if table == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table, field is required but not provided")
	}
	// Validate name
	googleSqlValid := utils.ValidateArgument(table.GetName(), utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlValid := utils.ValidateArgument(table.GetName(), utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", table.Name, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure schema is provided
	if table.GetSchema() == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema, field is required but not provided")
	}
	// Ensure columns are provided and not empty
	if table.GetSchema().GetColumns() == nil || len(table.GetSchema().GetColumns()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema.columns, field is required but not provided")
	}
	// Validate update_mask if provided
	if updateMask != nil && len(updateMask.GetPaths()) > 0 {
		// Normalize the update mask
		updateMask.Normalize()

		// Ensure only valid fields are updated i.e. schema.columns
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "schema.columns":

				// Validate columns
				for i, column := range table.GetSchema().GetColumns() {
					// Validate column name
					if valid := utils.ValidateArgument(column.Name, utils.SpannerGoogleSqlColumnIdRegex); !valid {
						return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].name (%s), must match `%s`", i, column.Name, utils.SpannerGoogleSqlColumnIdRegex)
					}

					// Ensure a type is provided
					if column.GetType() == "" {
						return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].type, field is required but not provided", i)
					}

					// If column type is PROTO ensure proto file descriptor set is provided
					if column.GetType() == schema.SpannerTableDataTypeProto.String() {
						if column.GetProtoFileDescriptorSet() == nil {
							return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set, field is required but not provided", i)
						}

						if column.GetProtoFileDescriptorSet().GetProtoPackage() == nil {
							return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set.proto_package, field is required but not provided", i)
						}

						//if column.ProtoFileDescriptorSet.FileDescriptorSetPath == nil {
						//	return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set.file_descriptor_set_path, field is required but not provided", i)
						//}

						//if column.ProtoFileDescriptorSet.FileDescriptorSetPathSource == ProtoFileDescriptorSetSourceUNSPECIFIED {
						//	return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set.file_descriptor_set_path_source, field is required but not not provided", i)
						//}
					}
				}

			default:
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid argument update_mask, only field `schema.columns` is allowed, got `%s`", path))
			}
		}
	}
	// If update mask is not provided, ensure allow missing is set
	if updateMask == nil && !allowMissing {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument allow_missing, must be true if update_mask is not provided")
	}

	// Decompose name to get project, instance, database and table
	nameParts := strings.Split(table.Name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	tableId := nameParts[7]

	// Get table state
	existingTable, err := s.GetSpannerTable(ctx, table.GetName())
	if err != nil {
		if status.Code(err) != codes.NotFound || errors.Is(err, schema.ErrTableNotFound{}) {
			return nil, err
		}
	}
	// If table does not exist and allow missing is set to false, return error
	if existingTable == nil && !allowMissing {
		return nil, status.Errorf(codes.NotFound, "Table %s not found, set allow_missing to true to create a new table", table.GetName())
	}
	// If backup exists, ensure update mask is provided
	if existingTable != nil && updateMask == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument update_mask, field is required but not provided")
	}

	// If table does not exist and allow missing is set, create the table
	if existingTable == nil {
		return s.CreateSpannerTable(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), tableId, table)
	}

	// Check if we have any PROTO columns and create the necessary proto bundles
	for _, column := range table.GetSchema().GetColumns() {
		if column.GetType() != schema.SpannerTableDataTypeProto.String() {
			continue
		}

		// If file descriptor set path is not provided, skip
		// In such cases, we assume the user has already created the proto bundle(s)
		if column.GetProtoFileDescriptorSet().GetFileDescriptorSetPath() == nil || column.GetProtoFileDescriptorSet().GetFileDescriptorSetPath().GetValue() == "" {
			continue
		}

		fdsBytes, err := fetchDescriptorSet(ctx, column.GetProtoFileDescriptorSet())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Error fetching proto file descriptor set: %v", err)
		}

		// Unmarshal the proto file descriptor set
		fds := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(fdsBytes, fds); err != nil {
			return nil, status.Errorf(codes.Internal, "Error unmarshalling proto file descriptor set: %v", err)
		}

		// Update the column proto file descriptor set with the fds
		column.GetProtoFileDescriptorSet().SetFileDescriptorSet(fds)

		// Create proto bundle
		if err := CreateProtoBundle(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), column.GetProtoFileDescriptorSet().GetProtoPackage().GetValue(), fdsBytes); err != nil {
			return nil, status.Errorf(codes.Internal, "Error creating proto bundle: %v", err)
		}
	}

	_, err = table.Update(ctx, existingTable)
	if err != nil {
		return nil, err
	}

	return table, nil
}

// DeleteSpannerTable deletes a Spanner table.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the table to delete.
//
// Returns: *emptypb.Empty
func (s *SpannerService) DeleteSpannerTable(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Get table state
	table, err := s.GetSpannerTable(ctx, name)
	if err != nil {
		if errors.Is(err, schema.ErrTableNotFound{}) {
			return nil, status.Errorf(codes.NotFound, "Table (%s) not found", name)
		}
		return nil, err
	}

	err = table.Delete(ctx)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *SpannerService) SetTableIamBinding(ctx context.Context, parent string, binding *TablePolicyBinding) (*TablePolicyBinding, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure binding is provided
	if binding == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding, field is required but not provided")
	}

	// Ensure role is provided
	if binding.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding.role, field is required but not provided")
	}

	// Ensure permissions are provided
	if binding.Permissions == nil || len(binding.Permissions) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding.permissions, field is required but not provided")
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	if err != nil {
		return nil, err
	}

	var permissions []string
	for _, permission := range binding.Permissions {
		permissions = append(permissions, permission.String())
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("GRANT %s ON TABLE %s TO ROLE %s", strings.Join(permissions, ", "), tableId, binding.Role))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("GRANT %s ON TABLE %s TO ROLE %s", strings.Join(permissions, ", "), tableId, binding.Role))
	}
	updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   database.GetName(),
		Statements: ddlStatements,
	})
	if err != nil {
		return nil, err
	}

	// Wait for LRO to complete
	err = updateDatabaseDdlOperation.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return binding, nil
}

func (s *SpannerService) GetTableIamBinding(ctx context.Context, parent string, role string) (*TablePolicyBinding, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure role is provided
	if role == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument role, field is required but not provided")
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	var rows []*TablePermissionsRow
	res := db.Raw("SELECT * FROM INFORMATION_SCHEMA.TABLE_PRIVILEGES WHERE table_name = ? AND grantee = ?", tableId, role).Scan(&rows)
	if res.Error != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table IAM binding: %v", res.Error)
	}

	if len(rows) == 0 {
		return nil, status.Errorf(codes.NotFound, "Table IAM binding %s not found", role)
	}

	binding := &TablePolicyBinding{
		Role: role,
	}
	for _, row := range rows {
		if row.GetPermission() == TablePolicyBindingPermission_UNSPECIFIED {
			continue
		}

		binding.Permissions = append(binding.Permissions, row.GetPermission())
	}

	return binding, nil
}

func (s *SpannerService) DeleteTableIamBinding(ctx context.Context, parent string, role string) error {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure role is provided
	if role == "" {
		return status.Error(codes.InvalidArgument, "Invalid argument role, field is required but not provided")
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	if err != nil {
		return err
	}

	// Get binding
	binding, err := s.GetTableIamBinding(ctx, parent, role)
	if err != nil {
		return err
	}

	var permissions []string
	for _, permission := range binding.Permissions {
		permissions = append(permissions, permission.String())
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("REVOKE %s ON TABLE %s FROM ROLE %s", strings.Join(permissions, ", "), tableId, role))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("REVOKE %s ON TABLE %s FROM ROLE %s", strings.Join(permissions, ", "), tableId, role))
	}
	updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   database.GetName(),
		Statements: ddlStatements,
	})
	if err != nil {
		return err
	}

	// Wait for LRO to complete
	err = updateDatabaseDdlOperation.Wait(ctx)
	if err != nil {
		return err
	}

	return nil
}

// CreateSpannerTableIndex creates a new Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that will serve the new index.
//   - index: *SpannerTableIndex - Required. The index to create.
//
// Returns: *SpannerTableIndex
func (s *SpannerService) CreateSpannerTableIndex(ctx context.Context, parent string, index *SpannerTableIndex) (*SpannerTableIndex, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure index is provided and has a name and columns
	if index == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument index, field is required but not provided")
	}
	googleSqlIndexIdValid := utils.ValidateArgument(index.Name, utils.SpannerGoogleSqlIndexIdRegex)
	postgresSqlIndexIdValid := utils.ValidateArgument(index.Name, utils.SpannerPostgresSqlIndexIdRegex)
	if !googleSqlIndexIdValid && !postgresSqlIndexIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index.name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", index.Name, utils.SpannerGoogleSqlIndexIdRegex, utils.SpannerPostgresSqlIndexIdRegex)
	}
	if index.Columns == nil || len(index.Columns) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument index.columns, field is required but not provided")
	}
	for i, column := range index.Columns {
		if column == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index.columns[%d], field is required but not provided", i)
		}

		googleSqlColumnIdValid := utils.ValidateArgument(column.Name, utils.SpannerGoogleSqlColumnIdRegex)
		postgresSqlColumnIdValid := utils.ValidateArgument(column.Name, utils.SpannerPostgresSqlColumnIdRegex)
		if !googleSqlColumnIdValid && !postgresSqlColumnIdValid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index.columns[%d].name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", i, column.Name, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
		}
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Create index
	err = CreateIndex(db, tableId, index)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating index: %v", err)
	}

	return index, nil
}

// GetSpannerTableIndex gets a Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that serves the index.
//   - name: string - Required. The name of the index to get.
//
// Returns: *SpannerTableIndex
func (s *SpannerService) GetSpannerTableIndex(ctx context.Context, parent string, name string) (*SpannerTableIndex, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Validate name
	googleSqlIndexIdValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlIndexIdRegex)
	postgresSqlIndexIdValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlIndexIdRegex)
	if !googleSqlIndexIdValid && !postgresSqlIndexIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlIndexIdRegex, utils.SpannerPostgresSqlIndexIdRegex)
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	indexes, err := GetIndexes(db, tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table indices: %v", err)
	}

	for _, index := range indexes {
		if index.Name == name {
			return index, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "Index %s not found", name)
}

// ListSpannerTableIndices lists Spanner table indices.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table whose indices should be listed.
//
// Returns: []*SpannerTableIndex
func (s *SpannerService) ListSpannerTableIndices(ctx context.Context, parent string) ([]*SpannerTableIndex, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	indexes, err := GetIndexes(db, tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table indices: %v", err)
	}

	return indexes, nil
}

// DeleteIndex deletes a Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that serves the index.
//   - indexName: string - Required. The name of the index to delete.
//
// Returns: *emptypb.Empty
func (s *SpannerService) DeleteSpannerTableIndex(ctx context.Context, parent string, indexName string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Validate index name
	googleSqlIndexIdValid := utils.ValidateArgument(indexName, utils.SpannerGoogleSqlIndexIdRegex)
	postgresSqlIndexIdValid := utils.ValidateArgument(indexName, utils.SpannerPostgresSqlIndexIdRegex)
	if !googleSqlIndexIdValid && !postgresSqlIndexIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index_name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", indexName, utils.SpannerGoogleSqlIndexIdRegex, utils.SpannerPostgresSqlIndexIdRegex)
	}

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	err = db.Migrator().DropIndex(tableId, indexName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error dropping index: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *SpannerService) CreateSpannerTableRowDeletionPolicy(ctx context.Context, parent string, ttl *SpannerTableRowDeletionPolicy) (*SpannerTableRowDeletionPolicy, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure ttl is provided and has a name, column and duration
	if ttl == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl, field is required but not provided")
	}
	if ttl.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.column, field is required but not provided")
	}
	googleSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerGoogleSqlColumnIdRegex)
	postgresSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerPostgresSqlColumnIdRegex)
	if !googleSqlColumnIdValid && !postgresSqlColumnIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument ttl.column (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", ttl.Column, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
	}
	if ttl.Duration == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field is required but not provided")
	}
	if ttl.Duration.GetValue() < 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field must be greater than or equal to 0")
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Create the deletion policy
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD ROW DELETION POLICY (OLDER_THAN(%s, INTERVAL %d DAY))", tableId, ttl.Column, ttl.Duration.GetValue())).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return ttl, nil
}

func (s *SpannerService) GetSpannerTableRowDeletionPolicy(ctx context.Context, parent string) (*SpannerTableRowDeletionPolicy, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	type RowDeletionPolicy struct {
		TABLE_NAME                     string
		ROW_DELETION_POLICY_EXPRESSION string
	}
	var policy *RowDeletionPolicy
	err = db.Raw("SELECT TABLE_NAME, ROW_DELETION_POLICY_EXPRESSION FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ? AND ROW_DELETION_POLICY_EXPRESSION IS NOT NULL", tableId).Scan(&policy).Error
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting row deletion policy: %v", err)
	}

	if policy == nil || policy.ROW_DELETION_POLICY_EXPRESSION == "" {
		return nil, status.Errorf(codes.NotFound, "Row deletion policy not found")
	}

	// Regular expression with capture groups
	re := regexp.MustCompile(`OLDER_THAN\((\w+),\s*INTERVAL\s+(\d+)\s+DAY\)`)

	// Find all matches and capture groups
	matches := re.FindStringSubmatch(policy.ROW_DELETION_POLICY_EXPRESSION)

	if len(matches) != 3 {
		return nil, status.Errorf(codes.Internal, "Error parsing row deletion policy: %v", err)
	}

	column := matches[1]
	durationStr := matches[2]
	duration, err := strconv.ParseInt(durationStr, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error parsing row deletion policy: %v", err)
	}

	return &SpannerTableRowDeletionPolicy{
		Column:   column,
		Duration: wrapperspb.Int64(duration),
	}, nil
}

func (s *SpannerService) UpdateSpannerTableRowDeletionPolicy(ctx context.Context, parent string, ttl *SpannerTableRowDeletionPolicy) (*SpannerTableRowDeletionPolicy, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure ttl is provided and has a name, column and duration
	if ttl == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl, field is required but not provided")
	}
	if ttl.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.column, field is required but not provided")
	}
	googleSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerGoogleSqlColumnIdRegex)
	postgresSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerPostgresSqlColumnIdRegex)
	if !googleSqlColumnIdValid && !postgresSqlColumnIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument ttl.column (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", ttl.Column, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
	}
	if ttl.Duration == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field is required but not provided")
	}
	if ttl.Duration.GetValue() < 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field must be greater than or equal to 0")
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Create the deletion policy
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s REPLACE ROW DELETION POLICY (OLDER_THAN(%s, INTERVAL %d DAY))", tableId, ttl.Column, ttl.Duration.GetValue())).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return ttl, nil
}

func (s *SpannerService) DeleteSpannerTableRowDeletionPolicy(ctx context.Context, parent string) error {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return err
	}

	// Create the deletion policy
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP ROW DELETION POLICY", tableId)).Error; err != nil {
		return status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return nil
}

func (s *SpannerService) CreateSpannerTableForeignKeyConstraint(ctx context.Context, parent string, constraint *schema.SpannerTableForeignKeyConstraint) (*schema.SpannerTableForeignKeyConstraint, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure constraint is provided and has a name and foreign keys
	if constraint == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint, field is required but not provided")
	}
	googleSqlConstraintIdValid := utils.ValidateArgument(constraint.Name, utils.SpannerGoogleSqlConstraintIdRegex)
	postgresSqlConstraintIdValid := utils.ValidateArgument(constraint.Name, utils.SpannerPostgresSqlConstraintIdRegex)
	if !googleSqlConstraintIdValid && !postgresSqlConstraintIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument constraint.name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", constraint.Name, utils.SpannerGoogleSqlConstraintIdRegex, utils.SpannerPostgresSqlConstraintIdRegex)
	}
	// Validate foreign key fields

	if constraint.ReferencedTable == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint.referenced_table, field is required but not provided")
	}
	googleSqlForeignKeyTableValid := utils.ValidateArgument(constraint.ReferencedTable, utils.SpannerGoogleSqlTableIdRegex)
	postgresSqlForeignKeyTableValid := utils.ValidateArgument(constraint.ReferencedTable, utils.SpannerPostgresSqlTableIdRegex)
	if !googleSqlForeignKeyTableValid && !postgresSqlForeignKeyTableValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument constraint.referenced_table (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", constraint.ReferencedTable, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	if constraint.ReferencedColumn == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint.referenced_column, field is required but not provided")
	}
	googleSqlForeignKeyColumnValid := utils.ValidateArgument(constraint.ReferencedColumn, utils.SpannerGoogleSqlColumnIdRegex)
	postgresSqlForeignKeyColumnValid := utils.ValidateArgument(constraint.ReferencedColumn, utils.SpannerPostgresSqlColumnIdRegex)
	if !googleSqlForeignKeyColumnValid && !postgresSqlForeignKeyColumnValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument constraint.referenced_column (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", constraint.ReferencedColumn, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
	}

	if constraint.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint.column, field is required but not provided")
	}
	googleSqlColumnValid := utils.ValidateArgument(constraint.Column, utils.SpannerGoogleSqlColumnIdRegex)
	postgresSqlColumnValid := utils.ValidateArgument(constraint.Column, utils.SpannerPostgresSqlColumnIdRegex)
	if !googleSqlColumnValid && !postgresSqlColumnValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument constraint.column (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", constraint.Column, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
	}

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	sqlStatement := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (`%s`) REFERENCES %s(`%s`)", tableId, constraint.Name, constraint.Column, constraint.ReferencedTable, constraint.ReferencedColumn)
	if constraint.OnDelete != schema.SpannerTableConstraintActionUnspecified {
		sqlStatement += fmt.Sprintf(" ON DELETE %s", constraint.OnDelete.String())
	}
	if err := db.Exec(sqlStatement).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating foreign key constraint: %v", err)
	}

	return constraint, nil
}

func (s *SpannerService) GetSpannerTableForeignKeyConstraint(ctx context.Context, parent string, name string) (*schema.SpannerTableForeignKeyConstraint, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Validate name
	googleSqlConstraintIdValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlConstraintIdRegex)
	postgresSqlConstraintIdValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlConstraintIdRegex)
	if !googleSqlConstraintIdValid && !postgresSqlConstraintIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlConstraintIdRegex, utils.SpannerPostgresSqlConstraintIdRegex)
	}

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	sqlStatement := `
	SELECT
	  TABLE_CONSTRAINTS.CONSTRAINT_NAME,
	  TABLE_CONSTRAINTS.TABLE_NAME AS CONSTRAINED_TABLE,
	  TABLE_CONSTRAINTS.CONSTRAINT_TYPE,
	  REFERENTIAL_CONSTRAINTS.UPDATE_RULE,
	  REFERENTIAL_CONSTRAINTS.DELETE_RULE,
	  KEY_COLUMN_USAGE.COLUMN_NAME AS CONSTRAINED_COLUMN,
	  UNIQUE_COLUMN_CONSTRAINT.TABLE_NAME AS REFERENCED_TABLE,
	  UNIQUE_COLUMN_CONSTRAINT.COLUMN_NAME AS REFERENCED_COLUMN
	FROM
	  INFORMATION_SCHEMA.TABLE_CONSTRAINTS
	INNER JOIN
	  INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS
	  ON TABLE_CONSTRAINTS.CONSTRAINT_NAME = REFERENTIAL_CONSTRAINTS.CONSTRAINT_NAME
	INNER JOIN
	  INFORMATION_SCHEMA.KEY_COLUMN_USAGE
	  ON TABLE_CONSTRAINTS.CONSTRAINT_NAME = KEY_COLUMN_USAGE.CONSTRAINT_NAME
	INNER JOIN
	  INFORMATION_SCHEMA.KEY_COLUMN_USAGE AS UNIQUE_COLUMN_CONSTRAINT
	  ON REFERENTIAL_CONSTRAINTS.UNIQUE_CONSTRAINT_NAME = UNIQUE_COLUMN_CONSTRAINT.CONSTRAINT_NAME
	  AND KEY_COLUMN_USAGE.POSITION_IN_UNIQUE_CONSTRAINT = UNIQUE_COLUMN_CONSTRAINT.ORDINAL_POSITION
	WHERE
	  TABLE_CONSTRAINTS.TABLE_NAME = ?
	  AND TABLE_CONSTRAINTS.CONSTRAINT_NAME = ?
	  AND TABLE_CONSTRAINTS.CONSTRAINT_TYPE = "FOREIGN KEY"
	ORDER BY
	  KEY_COLUMN_USAGE.ORDINAL_POSITION;
	`

	var result *Constraint
	db = db.Raw(sqlStatement, tableId, name).Scan(&result)
	if db.Error != nil {
		return nil, status.Errorf(codes.Internal, "Error getting foreign key constraint: %v", db.Error)
	}
	if result == nil {
		return nil, status.Errorf(codes.NotFound, "Foreign key constraint %s not found", name)
	}

	constaint := &schema.SpannerTableForeignKeyConstraint{
		Name:             result.CONSTRAINT_NAME,
		ReferencedTable:  result.REFERENCED_TABLE,
		ReferencedColumn: result.REFERENCED_COLUMN,
		Column:           result.CONSTRAINED_COLUMN,
		OnDelete:         schema.SpannerTableConstraintActionFromString(result.DELETE_RULE),
	}

	return constaint, nil
}

func (s *SpannerService) DeleteSpannerTableForeignKeyConstraint(ctx context.Context, parent string, name string) error {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Validate name
	googleSqlConstraintIdValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlConstraintIdRegex)
	postgresSqlConstraintIdValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlConstraintIdRegex)
	if !googleSqlConstraintIdValid && !postgresSqlConstraintIdValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlConstraintIdRegex, utils.SpannerPostgresSqlConstraintIdRegex)
	}

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	sqlStatement := fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", tableId, name)
	if err := db.Exec(sqlStatement).Error; err != nil {
		return status.Errorf(codes.Internal, "Error dropping foreign key constraint: %v", err)
	}

	return nil
}
