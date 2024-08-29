package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
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

// CreateSpannerDatabase creates a new Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the instance that will serve the new database.
//   - databaseId: string - Required. The ID of the database to create.
//   - database: *databasepb.Database - Required. The database to create.
//
// Returns: *databasepb.Database
func (s *SpannerService) CreateSpannerDatabase(ctx context.Context, parent string, databaseId string, database *databasepb.Database) (*databasepb.Database, error) {
	// Set database dialect to Google Standard SQL if not provided
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_DATABASE_DIALECT_UNSPECIFIED {
		database.DatabaseDialect = databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL
	}

	// Validate arguments
	// Validate database id
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		if valid := utils.ValidateArgument(databaseId, utils.SpannerGoogleSqlDatabaseIdRegex); !valid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument database_id (%s), must match `%s`", databaseId, utils.SpannerGoogleSqlDatabaseIdRegex)
		}
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		if valid := utils.ValidateArgument(databaseId, utils.SpannerPostgresSqlDatabaseIdRegex); !valid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument database_id (%s), must match `%s`", databaseId, utils.SpannerPostgresSqlDatabaseIdRegex)
		}
	}
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}
	// Ensure database is provided
	if database == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument database, field is required but not provided")
	}

	// Set database name
	database.Name = fmt.Sprintf("%s/databases/%s", parent, databaseId)

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Construct create statement
	var createStatement string
	var extraStatements []string
	switch database.GetDatabaseDialect() {
	case databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL:
		createStatement = fmt.Sprintf("CREATE DATABASE `%s`", databaseId)
		if database.GetVersionRetentionPeriod() != "" {
			extraStatements = append(extraStatements, fmt.Sprintf("ALTER DATABASE `%s` SET OPTIONS(version_retention_period='%s')", databaseId, database.GetVersionRetentionPeriod()))
		}
	case databasepb.DatabaseDialect_POSTGRESQL:
		createStatement = fmt.Sprintf("CREATE DATABASE \"%s\"", databaseId)
	}

	// Create database
	createDatabaseOperation, err := client.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:           parent,
		CreateStatement:  createStatement,
		ExtraStatements:  extraStatements,
		EncryptionConfig: database.GetEncryptionConfig(),
		DatabaseDialect:  database.GetDatabaseDialect(),
		ProtoDescriptors: nil,
	})
	if err != nil {
		return nil, err
	}

	// Wait for LRO to complete
	createdDatabase, err := createDatabaseOperation.Wait(ctx)
	if err != nil {
		return nil, err
	}

	// If dialect is PostgreSQL and version retention period is provided, update the database
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL && database.GetVersionRetentionPeriod() != "" {
		updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
			Database: createdDatabase.GetName(),
			Statements: []string{
				fmt.Sprintf("ALTER DATABASE %s SET spanner.version_retention_period=\"%s\"", databaseId, database.GetVersionRetentionPeriod()),
			},
		})
		if err != nil {
			// Drop database if update fails
			dropErr := client.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
				Database: createdDatabase.GetName(),
			})
			if dropErr != nil {
				return nil, dropErr
			}

			return nil, err
		}

		// Wait for LRO to complete
		err = updateDatabaseDdlOperation.Wait(ctx)
		if err != nil {
			return nil, err
		}

		// Get updated database
		createdDatabase, err = s.GetSpannerDatabase(ctx, createdDatabase.GetName())
		if err != nil {
			return nil, err
		}
	}

	return createdDatabase, nil
}

// GetSpannerDatabase gets a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the database to get.
//
// Returns: *databasepb.Database
func (s *SpannerService) GetSpannerDatabase(ctx context.Context, name string) (*databasepb.Database, error) {
	// Validate arguments
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}

	return database, nil
}

// ListSpannerDatabases lists Spanner databases in an instance.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The instance whose databases should be listed.
//   - pageSize: int32 - The maximum number of databases to return. Default is 0.
//   - pageToken: string - The value of `next_page_token` returned by a previous call.
//
// Returns: []*databasepb.Database
func (s *SpannerService) ListSpannerDatabases(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*databasepb.Database, string, error) {
	// Validate arguments
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}

	// "projects/my-project/instances/my-instance"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	var res []*databasepb.Database
	var nextPageToken string

	it := client.ListDatabases(ctx, &databasepb.ListDatabasesRequest{
		Parent:    parent,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	for {
		database, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		res = append(res, database)

		// Check if page size is reached
		if pageSize > 0 && len(res) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
	}

	return res, nextPageToken, nil
}

// UpdateSpannerDatabase updates a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - database: *databasepb.Database - Required. The database to update.
//   - updateMask: *fieldmaskpb.FieldMask - Required. The fields to update.
//   - allowMissing: bool - If true and the database does not exist, a new database will be created. Default is false.
//
// Returns: *databasepb.Database
func (s *SpannerService) UpdateSpannerDatabase(ctx context.Context, database *databasepb.Database, updateMask *fieldmaskpb.FieldMask) (*databasepb.Database, error) {
	// Validate arguments
	// Ensure database is provided
	if database == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument database, field is required but not provided")
	}
	// Validate name
	googleSqlValid := utils.ValidateArgument(database.GetName(), utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(database.GetName(), utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument database.name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", database.GetName(), utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}
	// Validate update_mask if provided
	if updateMask != nil && len(updateMask.GetPaths()) > 0 {
		// Normalize the update mask
		updateMask.Normalize()
		if valid := updateMask.IsValid(&databasepb.Database{}); !valid {
			return nil, status.Error(codes.InvalidArgument, "invalid update mask")
		}

		// Ensure only valid fields are updated i.e. enable_drop_protection, version_retention_period
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "enable_drop_protection":
			case "version_retention_period":
			default:
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid argument update_mask, only fields `enable_drop_protection` and `version_retention_period` are allowed, got `%s`", path))
			}
		}
	}
	// If update mask is not provided, return error
	if updateMask == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument update_mask, field is required but not provided")
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get database state
	_, err = client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: database.GetName(),
	})
	if err != nil {
		return nil, err
	}

	// Deconstruct backup name to get project, instance, cluster and backup id
	backupNameParts := strings.Split(database.GetName(), "/")
	databaseId := backupNameParts[5]

	// Update database
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "enable_drop_protection":
			updateDatabaseOperation, err := client.UpdateDatabase(ctx, &databasepb.UpdateDatabaseRequest{
				Database:   database,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"enable_drop_protection"}},
			})
			if err != nil {
				return nil, err
			}

			// Wait for LRO to complete
			_, err = updateDatabaseOperation.Wait(ctx)
			if err != nil {
				return nil, err
			}

		case "version_retention_period":
			var ddlStatements []string
			if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
				ddlStatements = append(ddlStatements, fmt.Sprintf("ALTER DATABASE `%s` SET OPTIONS(version_retention_period='%s')", databaseId, database.GetVersionRetentionPeriod()))
			}
			if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
				ddlStatements = append(ddlStatements, fmt.Sprintf("ALTER DATABASE %s SET spanner.version_retention_period=\"%s\"", databaseId, database.GetVersionRetentionPeriod()))
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
		}
	}

	// Get database state
	updatedDatabase, err := s.GetSpannerDatabase(ctx, database.GetName())
	if err != nil {
		return nil, err
	}

	return updatedDatabase, nil
}

// DeleteSpannerDatabase deletes a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the database to delete.
//
// Returns: *emptypb.Empty
func (s *SpannerService) DeleteSpannerDatabase(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Drop database
	err = client.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: name,
	})
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
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

// GetSpannerDatabaseIamPolicy gets the IAM policy for a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the database whose IAM policy to get.
//   - options: *iampb.GetPolicyOptions - Optional. Options for GetIamPolicy.
//
// Returns: *iampb.Policy
func (s *SpannerService) GetSpannerDatabaseIamPolicy(ctx context.Context, parent string, options *iampb.GetPolicyOptions) (*iampb.Policy, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	policy, err := client.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: parent,
		Options:  options,
	})
	if err != nil {
		return nil, err
	}

	return policy, nil
}

// SetSpannerDatabaseIamPolicy sets the IAM policy for a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the database whose IAM policy to set.
//   - policy: *iampb.Policy - Required. The IAM policy to set.
//   - updateMask: *fieldmaskpb.FieldMask - Optional. The fields to update.
//
// Returns: *iampb.Policy
func (s *SpannerService) SetSpannerDatabaseIamPolicy(ctx context.Context, parent string, policy *iampb.Policy, updateMask *fieldmaskpb.FieldMask) (*iampb.Policy, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}
	// If update mask is provided, validate it
	if updateMask != nil && len(updateMask.GetPaths()) > 0 {
		// Normalize the update mask
		updateMask.Normalize()
		if valid := updateMask.IsValid(&iampb.Policy{}); !valid {
			return nil, status.Error(codes.InvalidArgument, "invalid update mask")
		}
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	p, err := client.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
		Resource:   parent,
		Policy:     policy,
		UpdateMask: updateMask,
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

// TestSpannerDatabaseIamPermissions tests the specified permissions against a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the database to test access for.
//   - permissions: []string - Required. The set of permissions to check for the resource.
//
// Returns: []string
func (s *SpannerService) TestSpannerDatabaseIamPermissions(ctx context.Context, parent string, permissions []string) ([]string, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.TestIamPermissions(ctx, &iampb.TestIamPermissionsRequest{
		Resource:    parent,
		Permissions: permissions,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetPermissions(), nil
}

// CreateSpannerBackup creates a new Spanner database backup.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the instance that will serve the new backup.
//   - backupId: string - Required. The ID of the backup to create.
//   - backup: *databasepb.Backup - Required. The backup to create.
//   - encryptionConfig: *databasepb.CreateBackupEncryptionConfig - Optional. The encryption configuration for the backup.
//
// Returns: *databasepb.Backup
func (s *SpannerService) CreateSpannerBackup(ctx context.Context, parent string, backupId string, backup *databasepb.Backup, encryptionConfig *databasepb.CreateBackupEncryptionConfig) (*databasepb.Backup, error) {
	// Validate arguments
	// Validate parent name
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}
	// Validate backup id
	googleSqlValid := utils.ValidateArgument(backupId, utils.SpannerGoogleSqlBackupIdRegex)
	postgresSqlValid := utils.ValidateArgument(backupId, utils.SpannerPostgresSqlBackupIdRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup_id (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", backupId, utils.SpannerGoogleSqlBackupIdRegex, utils.SpannerPostgresSqlBackupIdRegex)
	}
	// Ensure backup is provided
	if backup == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup, field is required but not provided")
	}

	// Deconstruct parent name to get project, instance and cluster id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]

	// Set the backup name
	backup.Name = fmt.Sprintf("projects/%s/instances/%s/backups/%s", project, instance, backupId)
	b, err := s.GetSpannerBackup(ctx, backup.GetName(), nil)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}
	// If backup exists, return error
	if b != nil {
		return nil, status.Errorf(codes.AlreadyExists, "Backup %s already exists", backup.GetName())
	}

	// "projects/my-project/instances/my-instance/backups/my-backup"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	createBackupOperation, err := client.CreateBackup(ctx, &databasepb.CreateBackupRequest{
		Parent:           parent,
		BackupId:         backupId,
		Backup:           backup,
		EncryptionConfig: encryptionConfig,
	})
	if err != nil {
		return nil, err
	}

	createdBackup, err := createBackupOperation.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return createdBackup, nil
}

// GetSpannerBackup gets a Spanner database backup.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the backup to get.
//   - readMask: *fieldmaskpb.FieldMask - Optional. The fields to return.
//
// Returns: *databasepb.Backup
func (s *SpannerService) GetSpannerBackup(ctx context.Context, name string, readMask *fieldmaskpb.FieldMask) (*databasepb.Backup, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlBackupNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlBackupNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlBackupNameRegex, utils.SpannerPostgresSqlBackupNameRegex)
	}

	// "projects/my-project/instances/my-instance/backups/my-backup"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	backup, err := client.GetBackup(ctx, &databasepb.GetBackupRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}

	return backup, nil
}

// ListSpannerBackups lists Spanner backups in an instance.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The instance whose backups should be listed.
//   - filter: string - Optional. An expression for filtering the results of the request.
//   - pageSize: int32 - The maximum number of backups to return. Default is 0.
//   - pageToken: string - The value of `next_page_token` returned by a previous call.
//
// Returns: []*databasepb.Backup, string
func (s *SpannerService) ListSpannerBackups(ctx context.Context, parent string, filter string, pageSize int32, pageToken string) ([]*databasepb.Backup, string, error) {
	// Validate arguments
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}

	// "projects/my-project/instances/my-instance"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	var res []*databasepb.Backup
	var nextPageToken string

	it := client.ListBackups(ctx, &databasepb.ListBackupsRequest{
		Parent:    parent,
		Filter:    filter,
		PageSize:  pageSize,
		PageToken: pageToken,
	})

	for {
		backup, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		res = append(res, backup)

		// Check if page size is reached
		if pageSize > 0 && len(res) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
	}

	return res, nextPageToken, nil
}

// UpdateSpannerBackup updates a Spanner database backup.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - backup: *databasepb.Backup - Required. The backup to update.
//   - updateMask: *fieldmaskpb.FieldMask - Required. The fields to update.
//   - allowMissing: bool - If true and the backup does not exist, a new backup will be created. Default is false.
//
// Returns: *databasepb.Backup
func (s *SpannerService) UpdateSpannerBackup(ctx context.Context, backup *databasepb.Backup, updateMask *fieldmaskpb.FieldMask) (*databasepb.Backup, error) {
	// Validate arguments
	// Ensure backup is provided
	if backup == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument backup, field is required but not provided")
	}
	// Validate name
	googleSqlValid := utils.ValidateArgument(backup.GetName(), utils.SpannerGoogleSqlBackupNameRegex)
	postgresSqlValid := utils.ValidateArgument(backup.GetName(), utils.SpannerPostgresSqlBackupNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup.name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", backup.GetName(), utils.SpannerGoogleSqlBackupNameRegex, utils.SpannerPostgresSqlBackupNameRegex)
	}
	// Validate update_mask if provided
	if updateMask != nil && len(updateMask.GetPaths()) > 0 {
		// Normalize the update mask
		updateMask.Normalize()
		if valid := updateMask.IsValid(&databasepb.Backup{}); !valid {
			return nil, status.Error(codes.InvalidArgument, "invalid update mask")
		}

		// Ensure only valid fields are updated i.e. expire_time, version_time
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "expire_time":
			case "version_time":
			default:
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid argument update_mask, only fields `expire_time` and `version_time` is allowed, got `%s`", path))
			}
		}
	}
	// If update mask is not provided, return error
	if updateMask == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument update_mask, field is required but not provided")
	}

	// "projects/my-project/instances/my-instance/backups/my-backup"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get backup state
	_, err = client.GetBackup(ctx, &databasepb.GetBackupRequest{
		Name: backup.GetName(),
	})
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}

	// Update backup
	updatedBackup, err := client.UpdateBackup(ctx, &databasepb.UpdateBackupRequest{
		Backup:     backup,
		UpdateMask: updateMask,
	})
	if err != nil {
		return nil, err
	}

	return updatedBackup, nil
}

// DeleteSpannerBackup deletes a Spanner database backup.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the backup to delete.
//
// Returns: *emptypb.Empty
func (s *SpannerService) DeleteSpannerBackup(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlBackupNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlBackupNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlBackupNameRegex, utils.SpannerPostgresSqlBackupNameRegex)
	}

	// "projects/my-project/instances/my-instance/backups/my-backup"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Drop backup
	err = client.DeleteBackup(ctx, &databasepb.DeleteBackupRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
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
func (s *SpannerService) CreateSpannerTable(ctx context.Context, parent string, tableId string, table *SpannerTable) (*SpannerTable, error) {
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
	if table.Schema == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema, field is required but not provided")
	}
	// Ensure columns are provided and not empty
	if table.Schema == nil || table.Schema.Columns == nil || len(table.Schema.Columns) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema.columns, field is required but not provided")
	}
	// Validate columns
	for i, column := range table.Schema.Columns {
		// Validate column name
		if valid := utils.ValidateArgument(column.Name, utils.SpannerGoogleSqlColumnIdRegex); !valid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].name (%s), must match `%s`", i, column.Name, utils.SpannerGoogleSqlColumnIdRegex)
		}

		// Ensure a type is provided
		if column.Type == "" {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].type, field is required but not provided", i)
		}

		// If column type is PROTO ensure proto file descriptor set is provided
		if column.Type == SpannerTableDataType_PROTO.String() {
			if column.ProtoFileDescriptorSet == nil {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set, field is required but not provided", i)
			}

			if column.ProtoFileDescriptorSet.ProtoPackage == nil {
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

	// Check if we have any PROTO columns and create the necessary proto bundles
	for _, column := range table.Schema.Columns {
		if column.Type != SpannerTableDataType_PROTO.String() {
			continue
		}

		// If file descriptor set path is not provided, skip
		// In such cases, we assume the user has already created the proto bundle(s)
		if column.ProtoFileDescriptorSet.FileDescriptorSetPath == nil || column.ProtoFileDescriptorSet.FileDescriptorSetPath.GetValue() == "" {
			continue
		}

		fdsBytes, err := fetchDescriptorSet(ctx, column.ProtoFileDescriptorSet)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Error fetching proto file descriptor set: %v", err)
		}

		// Unmarshal the proto file descriptor set
		fds := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(fdsBytes, fds); err != nil {
			return nil, status.Errorf(codes.Internal, "Error unmarshalling proto file descriptor set: %v", err)
		}

		// Update the column proto file descriptor set with the fds
		column.ProtoFileDescriptorSet.fileDescriptorSet = fds

		// Create proto bundle
		if err := CreateProtoBundle(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), column.ProtoFileDescriptorSet.ProtoPackage.GetValue(), fdsBytes); err != nil {
			return nil, status.Errorf(codes.Internal, "Error creating proto bundle: %v", err)
		}

	}

	// Convert schema to struct
	structInstance, err := ParseSchemaToStruct(table.Schema)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error converting table.schema to struct: %v", err)
	}

	// Create table
	err = db.Table(tableId).Migrator().CreateTable(&structInstance)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating table: %v", err)
	}

	if err := UpdateColumnMetadata(ctx, db, tableId, table.Schema.Columns); err != nil {
		// This is not a fatal error, so we log it and continue
		tfLogger.Warn(ctx, fmt.Sprintf("Error updating column metadata table: %v", err))
		//return nil, status.Errorf(codes.Internal, "Error updating column metadata table: %v", err)
	}

	// Get created table
	updatedTable, err := s.GetSpannerTable(ctx, table.Name)
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
func (s *SpannerService) GetSpannerTable(ctx context.Context, name string) (*SpannerTable, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Decompose name to get project, instance, database and table
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	tableId := nameParts[7]

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

	columnTypes, err := db.Migrator().ColumnTypes(tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table columns: %v", err)
	}

	// If table does not exist, column types will be empty
	// Return not found error
	if columnTypes == nil || len(columnTypes) == 0 {
		return nil, status.Errorf(codes.NotFound, "Table %s not found", name)
	}

	// Get column metadata
	columnMetadata, err := GetColumnMetadata(db, tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting column metadata: %v", err)
	}
	columnMetadataMap := map[string]*ColumnMetadata{}
	for _, metadata := range columnMetadata {
		columnMetadataMap[metadata.ColumnName] = metadata
	}

	// Iterate over columns and add them to the schema
	columns := make([]*SpannerTableColumn, len(columnTypes))
	for i, columnType := range columnTypes {
		column := &SpannerTableColumn{
			Name: columnType.Name(),
		}

		if metadata, ok := columnMetadataMap[column.Name]; ok && metadata.Metadata != nil {

			// Populate primary keys if present
			switch metadata.Metadata.IsPrimaryKey {
			case "true":
				column.IsPrimaryKey = wrapperspb.Bool(true)
			case "false":
				column.IsPrimaryKey = wrapperspb.Bool(false)
			}

			// Populate computed if present
			switch metadata.Metadata.IsComputed {
			case "true":
				column.IsComputed = wrapperspb.Bool(true)
			case "false":
				column.IsComputed = wrapperspb.Bool(false)
			}

			// Populate computation ddl if present
			switch metadata.Metadata.ComputationDdl {
			case "nil":
			default:
				column.ComputationDdl = wrapperspb.String(metadata.Metadata.ComputationDdl)
			}

			// Populate auto increment
			switch metadata.Metadata.AutoIncrement {
			case "true":
				column.AutoIncrement = wrapperspb.Bool(true)
			case "false":
				column.AutoIncrement = wrapperspb.Bool(false)
			}

			// Populate unique if present
			switch metadata.Metadata.Unique {
			case "true":
				column.Unique = wrapperspb.Bool(true)
			case "false":
				column.Unique = wrapperspb.Bool(false)
			}

			// Populate auto create time if present
			switch metadata.Metadata.AutoCreateTime {
			case "true":
				column.AutoCreateTime = wrapperspb.Bool(true)
			case "false":
				column.AutoCreateTime = wrapperspb.Bool(false)
			}

			// Populate auto update time if present
			switch metadata.Metadata.AutoUpdateTime {
			case "true":
				column.AutoUpdateTime = wrapperspb.Bool(true)
			case "false":
				column.AutoUpdateTime = wrapperspb.Bool(false)
			}

			// Populate size if present
			switch metadata.Metadata.Size {
			case "nil":
			default:
				// Convert string to int64
				size, err := strconv.ParseInt(metadata.Metadata.Size, 10, 64)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Error converting size to int64: %v", err)
				}

				column.Size = wrapperspb.Int64(size)
			}

			// Populate precision and scale if present
			switch metadata.Metadata.Precision {
			case "nil":
			default:
				// Convert string to int64
				precision, err := strconv.ParseInt(metadata.Metadata.Precision, 10, 64)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Error converting precision to int64: %v", err)
				}

				column.Precision = wrapperspb.Int64(precision)
			}

			switch metadata.Metadata.Scale {
			case "nil":
			default:
				// Convert string to int64
				scale, err := strconv.ParseInt(metadata.Metadata.Scale, 10, 64)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Error converting scale to int64: %v", err)
				}

				column.Scale = wrapperspb.Int64(scale)
			}

			// Populate nullable if present
			switch metadata.Metadata.Required {
			case "true":
				column.Required = wrapperspb.Bool(true)
			case "false":
				column.Required = wrapperspb.Bool(false)
			}

			// Populate default value if present
			switch metadata.Metadata.DefaultValue {
			case "nil":
			default:
				column.DefaultValue = wrapperspb.String(metadata.Metadata.DefaultValue)
			}

			// Populate proto package
			var protoPackage string
			switch metadata.Metadata.ProtoPackage {
			case "nil":
			default:
				protoPackage = metadata.Metadata.ProtoPackage
			}

			// Populate proto file descriptor set
			var fileDescriptorSetPath string
			switch metadata.Metadata.FileDescriptorSetPath {
			case "nil":
			default:
				fileDescriptorSetPath = metadata.Metadata.FileDescriptorSetPath
			}

			// Populate ProtoFileDescriptorSet
			if protoPackage != "" || fileDescriptorSetPath != "" {
				column.ProtoFileDescriptorSet = &ProtoFileDescriptorSet{}

				if protoPackage != "" {
					column.ProtoFileDescriptorSet.ProtoPackage = wrapperspb.String(protoPackage)
				}

				if fileDescriptorSetPath != "" {
					column.ProtoFileDescriptorSet.FileDescriptorSetPath = wrapperspb.String(fileDescriptorSetPath)
				}
			}
		} else {
			// Populate primary keys if present
			if isPrimaryKey, ok := columnType.PrimaryKey(); ok {
				column.IsPrimaryKey = wrapperspb.Bool(isPrimaryKey)
			}
			// Populate auto increment
			if isAutoIncrement, ok := columnType.AutoIncrement(); ok {
				column.AutoIncrement = wrapperspb.Bool(isAutoIncrement)
			}
			// Populate unique if present
			if isUnique, ok := columnType.Unique(); ok {
				column.Unique = wrapperspb.Bool(isUnique)
			}
			// Populate size if present
			if size, ok := columnType.Length(); ok {
				column.Size = wrapperspb.Int64(size)
			}
			// Populate precision and scale if present
			if precision, scale, ok := columnType.DecimalSize(); ok {
				column.Precision = wrapperspb.Int64(precision)
				column.Scale = wrapperspb.Int64(scale)
			}
			// Populate nullable if present
			if nullable, ok := columnType.Nullable(); ok {
				column.Required = wrapperspb.Bool(!nullable)
			}
			//Populate default value if present
			if defaultValue, ok := columnType.DefaultValue(); ok {
				column.DefaultValue = wrapperspb.String(defaultValue)
			}

			if strings.HasPrefix(columnType.DatabaseTypeName(), "PROTO<") {
				// Set the proto file descriptor set
				column.ProtoFileDescriptorSet = &ProtoFileDescriptorSet{
					ProtoPackage:                wrapperspb.String(strings.TrimSuffix(strings.TrimPrefix(columnType.DatabaseTypeName(), "PROTO<"), ">")),
					FileDescriptorSetPath:       nil,
					FileDescriptorSetPathSource: 0,
					fileDescriptorSet:           nil,
				}
			}

			if strings.HasPrefix(columnType.DatabaseTypeName(), "ENUM<") {
				// Set the proto file descriptor set
				column.ProtoFileDescriptorSet = &ProtoFileDescriptorSet{
					ProtoPackage:                wrapperspb.String(strings.TrimSuffix(strings.TrimPrefix(columnType.DatabaseTypeName(), "ENUM<"), ">")),
					FileDescriptorSetPath:       nil,
					FileDescriptorSetPathSource: 0,
					fileDescriptorSet:           nil,
				}
			}
		}

		column.Type = columnType.DatabaseTypeName()
		if strings.HasPrefix(columnType.DatabaseTypeName(), "PROTO<") || strings.HasPrefix(columnType.DatabaseTypeName(), "ENUM<") {
			// Set the correct column type
			// PROTO<my_package.MyMessage>
			column.Type = SpannerTableDataType_PROTO.String()
		}

		columns[i] = column
	}

	table := &SpannerTable{
		Name: name,
		Schema: &SpannerTableSchema{
			Columns: columns,
		},
	}

	return table, nil
}

// ListSpannerTables lists Spanner tables in a database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The database whose tables should be listed.
//
// Returns: []*SpannerTable
func (s *SpannerService) ListSpannerTables(ctx context.Context, parent string) ([]*SpannerTable, error) {
	// Validate arguments
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// Decompose parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]

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

	tableNames, err := db.Migrator().GetTables()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting tables: %v", err)
	}

	res := make([]*SpannerTable, len(tableNames))

	for i, tableName := range tableNames {
		table, err := s.GetSpannerTable(ctx, fmt.Sprintf("%s/tables/%s", parent, tableName))
		if err != nil {
			return nil, err
		}

		res[i] = table
	}

	return res, nil
}

// UpdateSpannerTable updates a Spanner table.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - table: *SpannerTable - Required. The table to update.
//   - allowMissing: bool - If true and the table does not exist, a new table will be created. Default is false.
//
// Returns: *SpannerTable
func (s *SpannerService) UpdateSpannerTable(ctx context.Context, table *SpannerTable, updateMask *fieldmaskpb.FieldMask, allowMissing bool) (*SpannerTable, error) {
	// Validate arguments
	// Ensure table is provided
	if table == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table, field is required but not provided")
	}
	// Validate name
	googleSqlValid := utils.ValidateArgument(table.Name, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlValid := utils.ValidateArgument(table.Name, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", table.Name, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure schema is provided
	if table.Schema == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table.schema, field is required but not provided")
	}
	// Ensure columns are provided and not empty
	if table.Schema.Columns == nil || len(table.Schema.Columns) == 0 {
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
				for i, column := range table.Schema.Columns {
					// Validate column name
					if valid := utils.ValidateArgument(column.Name, utils.SpannerGoogleSqlColumnIdRegex); !valid {
						return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].name (%s), must match `%s`", i, column.Name, utils.SpannerGoogleSqlColumnIdRegex)
					}

					// Ensure a type is provided
					if column.Type == "" {
						return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].type, field is required but not provided", i)
					}

					// If column type is PROTO ensure proto file descriptor set is provided
					if column.Type == SpannerTableDataType_PROTO.String() {
						if column.ProtoFileDescriptorSet == nil {
							return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_file_descriptor_set, field is required but not provided", i)
						}

						if column.ProtoFileDescriptorSet.ProtoPackage == nil {
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
	existingTable, err := s.GetSpannerTable(ctx, table.Name)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}
	// If table does not exist and allow missing is set to false, return error
	if existingTable == nil && !allowMissing {
		return nil, status.Errorf(codes.NotFound, "Table %s not found, set allow_missing to true to create a new table", table.Name)
	}
	// If backup exists, ensure update mask is provided
	if existingTable != nil && updateMask == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument update_mask, field is required but not provided")
	}

	// If table does not exist and allow missing is set, create the table
	if existingTable == nil {
		return s.CreateSpannerTable(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), tableId, table)
	}

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

	// Check if we have any PROTO columns and create the necessary proto bundles
	for _, column := range table.Schema.Columns {
		if column.Type != SpannerTableDataType_PROTO.String() {
			continue
		}

		// If file descriptor set path is not provided, skip
		// In such cases, we assume the user has already created the proto bundle(s)
		if column.ProtoFileDescriptorSet.FileDescriptorSetPath == nil || column.ProtoFileDescriptorSet.FileDescriptorSetPath.GetValue() == "" {
			continue
		}

		fdsBytes, err := fetchDescriptorSet(ctx, column.ProtoFileDescriptorSet)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Error fetching proto file descriptor set: %v", err)
		}

		// Unmarshal the proto file descriptor set
		fds := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(fdsBytes, fds); err != nil {
			return nil, status.Errorf(codes.Internal, "Error unmarshalling proto file descriptor set: %v", err)
		}

		// Update the column proto file descriptor set with the fds
		column.ProtoFileDescriptorSet.fileDescriptorSet = fds

		// Create proto bundle
		if err := CreateProtoBundle(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), column.ProtoFileDescriptorSet.ProtoPackage.GetValue(), fdsBytes); err != nil {
			return nil, status.Errorf(codes.Internal, "Error creating proto bundle: %v", err)
		}
	}

	// Convert schema to struct
	structInstance, err := ParseSchemaToStruct(table.Schema)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error converting table.schema to struct: %v", err)
	}

	// Iterate over update mask and update columns
	var addedColumns []*SpannerTableColumn
	var removedColumns []*SpannerTableColumn
	var updatedColumns []*SpannerTableColumn

	// Get existing columns
	existingColumns := map[string]*SpannerTableColumn{}
	for _, column := range existingTable.Schema.Columns {
		existingColumns[column.Name] = column
	}
	newColumns := map[string]*SpannerTableColumn{}
	for _, column := range table.Schema.Columns {
		newColumns[column.Name] = column
	}

	// If there are no existing columns, but new columns are provided, add all new columns
	if (existingColumns == nil || len(existingColumns) == 0) && (newColumns != nil && len(newColumns) > 0) {
		for _, column := range newColumns {
			addedColumns = append(addedColumns, column)
		}
	}

	// If there are existing columns and new columns are provided, compare and update
	if (existingColumns != nil && len(existingColumns) > 0) && (newColumns != nil && len(newColumns) > 0) {
		// Iterate over the new columns and compare with the existing column families
		for _, newColumn := range newColumns {
			if _, exists := existingColumns[newColumn.Name]; !exists {
				// Column does not exist in existing columns, add it
				addedColumns = append(addedColumns, newColumn)
			}
		}
	}

	// If there are existing columns, but no new columns are provided, remove all existing columns
	if (existingColumns != nil && len(existingColumns) > 0) && (newColumns == nil || len(newColumns) == 0) {
		for _, column := range existingColumns {
			removedColumns = append(removedColumns, column)
		}
	}

	// If there are existing columns and new columns are provided, compare and update
	if (existingColumns != nil && len(existingColumns) > 0) && (newColumns != nil && len(newColumns) > 0) {
		// Iterate over the existing columns and compare with the new column families
		for _, existingColumn := range existingColumns {
			if _, exists := newColumns[existingColumn.Name]; !exists {
				// Column does not exist in new columns, remove it
				removedColumns = append(removedColumns, existingColumn)
			}
		}
	}

	// If there are existing columns and new columns are provided, compare and update
	if (existingColumns != nil && len(existingColumns) > 0) && (newColumns != nil && len(newColumns) > 0) {
		// Iterate over the existing columns and compare with the new column families
		for _, existingColumn := range existingColumns {
			if newColumn, exists := newColumns[existingColumn.Name]; exists {
				// Column exists in new columns, update it
				updatedColumns = append(updatedColumns, newColumn)
			}
		}
	}

	// If there are removed columns, drop them
	if len(removedColumns) > 0 {
		// Sort removedColumns so that computed columns are deleted first
		sort.SliceStable(removedColumns, func(i, j int) bool {
			if removedColumns[i].IsComputed == nil {
				return false
			}

			return removedColumns[i].IsComputed.GetValue()
		})

		for _, column := range removedColumns {
			err = db.Table(tableId).Migrator().DropColumn(&structInstance, column.Name)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Error dropping column: %v", err)
			}
		}

		if err := DeleteColumnMetadata(db, tableId, removedColumns); err != nil {
			return nil, status.Errorf(codes.Internal, "Error deleting column metadata: %v", err)
		}
	}

	// Migrate table
	err = db.Table(tableId).Migrator().AutoMigrate(&structInstance)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error updating table: %v", err)
	}

	if err := UpdateColumnMetadata(ctx, db, tableId, table.Schema.Columns); err != nil {
		// This is not a fatal error, so we log it and continue
		tfLogger.Warn(ctx, fmt.Sprintf("Error updating column metadata table: %v", err))
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

	// Decompose name to get project, instance, database and table
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	tableId := nameParts[7]

	// Get table state
	_, err := s.GetSpannerTable(ctx, name)
	if err != nil {
		return nil, err
	}

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

	// Drop table
	err = db.Migrator().DropTable(tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error dropping table: %v", err)
	}

	// Delete column metadata
	if err := DeleteColumnMetadata(db, tableId, []*SpannerTableColumn{}); err != nil {
		return nil, status.Errorf(codes.Internal, "Error deleting column metadata: %v", err)
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

	err = db.Migrator().DropIndex(tableId, indexName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error dropping index: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *SpannerService) CreateSpannerTableForeignKeyConstraint(ctx context.Context, parent string, constraint *SpannerTableForeignKeyConstraint) (*SpannerTableForeignKeyConstraint, error) {
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

	sqlStatement := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)", tableId, constraint.Name, constraint.Column, constraint.ReferencedTable, constraint.ReferencedColumn)
	if constraint.OnDelete != SpannerTableForeignKeyConstraintActionUnspecified {
		sqlStatement += fmt.Sprintf(" ON DELETE %s", constraint.OnDelete.String())
	}
	if err := db.Exec(sqlStatement).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating foreign key constraint: %v", err)
	}

	return constraint, nil
}

func (s *SpannerService) GetSpannerTableForeignKeyConstraint(ctx context.Context, parent string, name string) (*SpannerTableForeignKeyConstraint, error) {
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

	sqlStatement := `
	SELECT
	  TABLE_CONSTRAINTS.CONSTRAINT_NAME,
	  TABLE_CONSTRAINTS.TABLE_NAME,
	  TABLE_CONSTRAINTS.CONSTRAINT_TYPE,
	  REFERENTIAL_CONSTRAINTS.UPDATE_RULE,
	  REFERENTIAL_CONSTRAINTS.DELETE_RULE
	FROM
	  INFORMATION_SCHEMA.TABLE_CONSTRAINTS
	INNER JOIN
	  INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS
	ON
	  TABLE_CONSTRAINTS.CONSTRAINT_NAME = REFERENTIAL_CONSTRAINTS.CONSTRAINT_NAME
	WHERE TABLE_CONSTRAINTS.TABLE_NAME = ? and TABLE_CONSTRAINTS.CONSTRAINT_NAME = ? AND TABLE_CONSTRAINTS.CONSTRAINT_TYPE = "FOREIGN KEY"
	`

	var result *Constraint
	db = db.Raw(sqlStatement, tableId, name).Scan(&result)
	if db.Error != nil {
		return nil, status.Errorf(codes.Internal, "Error getting foreign key constraint: %v", db.Error)
	}
	if result == nil {
		return nil, status.Errorf(codes.NotFound, "Foreign key constraint %s not found", name)
	}

	return nil, nil
}
