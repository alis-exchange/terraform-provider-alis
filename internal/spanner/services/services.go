package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	databaseApiV1 "cloud.google.com/go/spanner/admin/database/apiv1"
	spanner "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	spannergorm "github.com/googleapis/go-gorm-spanner"
	_ "github.com/googleapis/go-sql-spanner"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
	"terraform-provider-alis/internal/utils"
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

// SpannerTableColumn represents a Spanner table column.
type SpannerTableColumn struct {
	// The name of the column.
	//
	// Must be unique within the table.
	Name string
	// Whether the column is a primary key
	IsPrimaryKey *wrapperspb.BoolValue
	// Whether the column is auto-incrementing
	// This is typically paired with is_primary_key=true
	// This is only valid for numeric columns i.e. INT64, FLOAT64
	AutoIncrement *wrapperspb.BoolValue
	// Whether the values in the column are unique
	Unique *wrapperspb.BoolValue
	// The type of the column
	Type string
	// The maximum size of the column.
	//
	// For STRING columns, this is the maximum length of the column in characters.
	// For BYTES columns, this is the maximum length of the column in bytes.
	Size *wrapperspb.Int64Value
	// The precision of the column.
	// This is typically paired with numeric columns i.e. INT64, FLOAT64
	Precision *wrapperspb.Int64Value
	// The scale of the column.
	// This is typically paired with numeric columns i.e. INT64, FLOAT64
	Scale *wrapperspb.Int64Value
	// Whether the column is nullable
	Required *wrapperspb.BoolValue
	// The default value of the column.
	//
	// Accepts any type of value given that the value is valid for the column type.
	DefaultValue *wrapperspb.StringValue
}

// SpannerTableIndex represents a Spanner table index.
type SpannerTableIndex struct {
	// The name of the index
	Name string
	// The columns that make up the index
	Columns []string
	// Whether the index is unique
	Unique *wrapperspb.BoolValue
}

// SpannerTableSchema represents the schema of a Spanner table.
type SpannerTableSchema struct {
	// The columns that make up the table schema.
	Columns []*SpannerTableColumn
	// The indexes for the table.
	Indices []*SpannerTableIndex
}

// SpannerTable represents a Spanner table.
type SpannerTable struct {
	// Fully qualified name of the table.
	// Format: projects/{project}/instances/{instance}/databases/{database}/tables/{table}
	Name string
	// The schema of the table.
	Schema *SpannerTableSchema
}

// DatabaseOperationResponse represents a database operation response.
type DatabaseOperationResponse[T interface{}] struct {
	Operation       *T
	CloseClientConn func() error
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
func CreateSpannerDatabase(ctx context.Context, parent string, databaseId string, database *databasepb.Database) (*DatabaseOperationResponse[databaseApiV1.CreateDatabaseOperation], error) {
	// Validate arguments
	// Validate database id
	if valid := utils.ValidateArgument(databaseId, utils.SpannerDatabaseIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument database_id (%s), must match `%s`", databaseId, utils.SpannerDatabaseIdRegex)
	}
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}
	// Ensure database is provided
	if database == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument database, field is required but not provided")
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}

	// Construct create statement
	createStatement := fmt.Sprintf("CREATE DATABASE `%s`", databaseId)
	var extraStatements []string
	if database.GetVersionRetentionPeriod() != "" {
		extraStatements = append(extraStatements, fmt.Sprintf("ALTER DATABASE `%s` SET OPTIONS(version_retention_period='%s')", databaseId, database.GetVersionRetentionPeriod()))
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

	return &DatabaseOperationResponse[databaseApiV1.CreateDatabaseOperation]{
		Operation: createDatabaseOperation,
		CloseClientConn: func() error {
			return client.Close()
		},
	}, nil
}

// GetSpannerDatabase gets a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the database to get.
//
// Returns: *databasepb.Database
func GetSpannerDatabase(ctx context.Context, name string) (*databasepb.Database, error) {
	// Validate arguments
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerDatabaseNameRegex)
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
func ListSpannerDatabases(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*databasepb.Database, string, error) {
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
func UpdateSpannerDatabase(ctx context.Context, database *databasepb.Database, updateMask *fieldmaskpb.FieldMask) (*databasepb.Database, error) {
	// Validate arguments
	// Ensure database is provided
	if database == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument database, field is required but not provided")
	}
	// Validate name
	if valid := utils.ValidateArgument(database.GetName(), utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument database.name (%s), must match `%s`", database.GetName(), utils.SpannerDatabaseNameRegex)
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
			updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
				Database: database.GetName(),
				Statements: []string{
					fmt.Sprintf("ALTER DATABASE `%s` SET OPTIONS(version_retention_period='%s')", databaseId, database.GetVersionRetentionPeriod()),
				},
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
	updatedDatabase, err := GetSpannerDatabase(ctx, database.GetName())
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
func DeleteSpannerDatabase(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerDatabaseNameRegex)
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

// GetSpannerDatabaseIamPolicy gets the IAM policy for a Spanner database.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the database whose IAM policy to get.
//   - options: *iampb.GetPolicyOptions - Optional. Options for GetIamPolicy.
//
// Returns: *iampb.Policy
func GetSpannerDatabaseIamPolicy(ctx context.Context, parent string, options *iampb.GetPolicyOptions) (*iampb.Policy, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
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
func SetSpannerDatabaseIamPolicy(ctx context.Context, parent string, policy *iampb.Policy, updateMask *fieldmaskpb.FieldMask) (*iampb.Policy, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
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
func TestSpannerDatabaseIamPermissions(ctx context.Context, parent string, permissions []string) ([]string, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
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
func CreateSpannerBackup(ctx context.Context, parent string, backupId string, backup *databasepb.Backup, encryptionConfig *databasepb.CreateBackupEncryptionConfig) (*DatabaseOperationResponse[databaseApiV1.CreateBackupOperation], error) {
	// Validate arguments
	// Validate parent name
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}
	// Validate backup id
	if valid := utils.ValidateArgument(backupId, utils.SpannerBackupIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup_id (%s), must match `%s`", backupId, utils.SpannerBackupIdRegex)
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
	b, err := GetSpannerBackup(ctx, backup.GetName(), nil)
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

	createBackupOperation, err := client.CreateBackup(ctx, &databasepb.CreateBackupRequest{
		Parent:           parent,
		BackupId:         backupId,
		Backup:           backup,
		EncryptionConfig: encryptionConfig,
	})
	if err != nil {
		return nil, err
	}

	return &DatabaseOperationResponse[databaseApiV1.CreateBackupOperation]{
		Operation: createBackupOperation,
		CloseClientConn: func() error {
			return client.Close()
		},
	}, nil
}

// GetSpannerBackup gets a Spanner database backup.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - name: string - Required. The name of the backup to get.
//   - readMask: *fieldmaskpb.FieldMask - Optional. The fields to return.
//
// Returns: *databasepb.Backup
func GetSpannerBackup(ctx context.Context, name string, readMask *fieldmaskpb.FieldMask) (*databasepb.Backup, error) {
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerBackupNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerBackupNameRegex)
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
func ListSpannerBackups(ctx context.Context, parent string, filter string, pageSize int32, pageToken string) ([]*databasepb.Backup, string, error) {
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
func UpdateSpannerBackup(ctx context.Context, backup *databasepb.Backup, updateMask *fieldmaskpb.FieldMask) (*databasepb.Backup, error) {
	// Validate arguments
	// Ensure backup is provided
	if backup == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument backup, field is required but not provided")
	}
	// Validate name
	if valid := utils.ValidateArgument(backup.GetName(), utils.SpannerBackupNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", backup.GetName(), utils.SpannerBackupNameRegex)
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
func DeleteSpannerBackup(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerBackupNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerBackupNameRegex)
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
func CreateSpannerTable(ctx context.Context, parent string, tableId string, table *SpannerTable) (*SpannerTable, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
	}
	// Validate table id
	if valid := utils.ValidateArgument(tableId, utils.SpannerTableIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table_id (%s), must match `%s`", tableId, utils.SpannerTableIdRegex)
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
		if valid := utils.ValidateArgument(column.Name, utils.SpannerColumnIdRegex); !valid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].name (%s), must match `%s`", i, column.Name, utils.SpannerColumnIdRegex)
		}

		// Ensure a type is provided
		if column.Type == "" {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].type, field is required but not provided", i)
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
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
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

	// Get created table
	updatedTable, err := GetSpannerTable(ctx, table.Name)
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
func GetSpannerTable(ctx context.Context, name string) (*SpannerTable, error) {
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerTableNameRegex)
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

	// Iterate over columns and add them to the schema
	columns := make([]*SpannerTableColumn, len(columnTypes))
	for i, columnType := range columnTypes {
		column := &SpannerTableColumn{
			Name: columnType.Name(),
		}

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

		column.Type = columnType.DatabaseTypeName()

		columns[i] = column
	}

	var indices []*SpannerTableIndex

	indexes, err := GetIndexes(db, tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table indices: %v", err)
	}

	for _, index := range indexes {
		if primaryKey, _ := index.PrimaryKey(); primaryKey {
			continue
		}

		idx := &SpannerTableIndex{
			Name:    index.Name(),
			Columns: index.Columns(),
		}
		if unique, ok := index.Unique(); ok {
			idx.Unique = wrapperspb.Bool(unique)
		}

		indices = append(indices, idx)
	}

	table := &SpannerTable{
		Name: name,
		Schema: &SpannerTableSchema{
			Columns: columns,
			Indices: indices,
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
func ListSpannerTables(ctx context.Context, parent string) ([]*SpannerTable, error) {
	// Validate arguments
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerDatabaseNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
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
		table, err := GetSpannerTable(ctx, fmt.Sprintf("%s/tables/%s", parent, tableName))
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
func UpdateSpannerTable(ctx context.Context, table *SpannerTable, updateMask *fieldmaskpb.FieldMask, allowMissing bool) (*SpannerTable, error) {
	// Validate arguments
	// Ensure table is provided
	if table == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument table, field is required but not provided")
	}
	// Validate name
	if valid := utils.ValidateArgument(table.Name, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", table.Name, utils.SpannerTableNameRegex)
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

		// Ensure only valid fields are updated i.e. schema.columns, schema.indices
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "schema.columns":
			case "schema.indices":
			default:
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid argument update_mask, only fields `schema.columns` and `schema.indices` are allowed, got `%s`", path))
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
	existingTable, err := GetSpannerTable(ctx, table.Name)
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
		return CreateSpannerTable(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId), tableId, table)
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
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}

	// Convert schema to struct
	structInstance, err := ParseSchemaToStruct(table.Schema)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error converting table.schema to struct: %v", err)
	}

	// Iterate over update mask and update columns
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "schema.columns":
			var removedColumns []*SpannerTableColumn

			// Get existing columns
			existingColumns := existingTable.Schema.Columns
			newColumns := map[string]*SpannerTableColumn{}
			for _, column := range table.Schema.Columns {
				newColumns[column.Name] = column
			}

			// If there are existing columns, but no new columns are provided, remove all existing columns
			if (existingColumns != nil && len(existingColumns) > 0) && (newColumns == nil || len(newColumns) == 0) {
				for _, column := range existingColumns {
					removedColumns = append(removedColumns, column)
				}
			}

			// If there are existing columns and new columns are provided, compare and update
			if (existingColumns != nil && len(existingColumns) > 0) && (newColumns != nil && len(newColumns) > 0) {
				// Iterate over the existing column families and compare with the new column families
				for _, existingColumn := range existingColumns {
					if _, exists := newColumns[existingColumn.Name]; !exists {
						// Column family does not exist in new column families, remove it
						removedColumns = append(removedColumns, existingColumn)
					}
				}
			}

			// If there are removed columns, drop them
			if len(removedColumns) > 0 {
				for _, column := range removedColumns {
					err = db.Table(tableId).Migrator().DropColumn(&structInstance, column.Name)
					if err != nil {
						return nil, status.Errorf(codes.Internal, "Error dropping column: %v", err)
					}
				}
			}
		case "schema.indices":
			var removedIndices []*SpannerTableIndex

			// Get existing indices
			existingIndices := existingTable.Schema.Indices
			newIndices := map[string]*SpannerTableIndex{}
			for _, index := range table.Schema.Indices {
				newIndices[index.Name] = index
			}

			// If there are existing indices, but no new indices are provided, remove all existing indices
			if (existingIndices != nil && len(existingIndices) > 0) && (newIndices == nil || len(newIndices) == 0) {
				for _, index := range existingIndices {
					removedIndices = append(removedIndices, index)
				}
			}

			// If there are existing indices and new indices are provided, compare and update
			if (existingIndices != nil && len(existingIndices) > 0) && (newIndices != nil && len(newIndices) > 0) {
				// Iterate over the existing indices and compare with the new indices
				for _, existingIndex := range existingIndices {
					if _, exists := newIndices[existingIndex.Name]; !exists {
						// Index does not exist in new indices, remove it
						removedIndices = append(removedIndices, existingIndex)
					}
				}
			}

			// If there are removed indices, drop them
			if len(removedIndices) > 0 {
				for _, index := range removedIndices {
					err = db.Migrator().DropIndex(tableId, index.Name)
					if err != nil {
						return nil, status.Errorf(codes.Internal, "Error dropping index: %v", err)
					}
				}
			}
		}
	}

	// Migrate table
	err = db.Table(tableId).Migrator().AutoMigrate(&structInstance)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error updating table: %v", err)
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
func DeleteSpannerTable(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerTableNameRegex)
	}

	// Decompose name to get project, instance, database and table
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	tableId := nameParts[7]

	// Get table state
	table, err := GetSpannerTable(ctx, name)
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
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}

	// If table has any indices, drop them
	if table.Schema != nil && table.Schema.Indices != nil && len(table.Schema.Indices) > 0 {
		for _, index := range table.Schema.Indices {
			err = db.Migrator().DropIndex(tableId, index.Name)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Error dropping index: %v", err)
			}
		}
	}

	// If table has any rows, delete them
	result := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Exec(fmt.Sprintf("DELETE FROM %s WHERE TRUE", tableId))
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "Error deleting rows: %v", db.Error)
	}

	// Drop table
	err = db.Migrator().DropTable(tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error dropping table: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// CreateSpannerTableIndex creates a new Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that will serve the new index.
//   - index: *SpannerTableIndex - Required. The index to create.
//
// Returns: *SpannerTableIndex
func CreateSpannerTableIndex(ctx context.Context, parent string, index *SpannerTableIndex) (*SpannerTableIndex, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
	}
	// Ensure index is provided and has a name and columns
	if index == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument index, field is required but not provided")
	}
	if valid := utils.ValidateArgument(index.Name, utils.SpannerIndexIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index.name (%s), must match `%s`", index.Name, utils.SpannerIndexIdRegex)
	}
	if index.Columns == nil || len(index.Columns) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument index.columns, field is required but not provided")
	}
	for i, column := range index.Columns {
		if valid := utils.ValidateArgument(column, utils.SpannerColumnIdRegex); !valid {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index.columns[%d] (%s), must match `%s`", i, column, utils.SpannerColumnIdRegex)
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
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}

	// Get parent table
	table, err := GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Convert schema to struct
	structInstance, err := ParseSchemaToStruct(table.Schema)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error converting table.schema to struct: %v", err)
	}

	// Check if index already exists
	indexExists := db.Table(tableId).Migrator().HasIndex(&structInstance, index.Name)
	if indexExists {
		return nil, status.Errorf(codes.AlreadyExists, "Index %s already exists", index.Name)
	}

	table.Schema.Indices = append(table.Schema.Indices, index)

	// Construct a new dynamic struct with the updated schema
	structInstance, err = ParseSchemaToStruct(table.Schema)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error converting table.schema to struct: %v", err)
	}

	// Create index
	err = db.Table("").Migrator().CreateIndex(&structInstance, index.Name)
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
func GetSpannerTableIndex(ctx context.Context, parent string, name string) (*SpannerTableIndex, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
	}
	// Validate name
	if valid := utils.ValidateArgument(name, utils.SpannerIndexIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.SpannerIndexIdRegex)
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
		if index.Name() == name {
			columns := index.Columns()

			idx := &SpannerTableIndex{
				Name:    name,
				Columns: columns,
			}
			if unique, ok := index.Unique(); ok {
				idx.Unique = wrapperspb.Bool(unique)
			}

			return idx, nil
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
func ListSpannerTableIndices(ctx context.Context, parent string) ([]*SpannerTableIndex, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerDatabaseNameRegex)
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
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}

	indexes, err := GetIndexes(db, tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table indices: %v", err)
	}

	res := make([]*SpannerTableIndex, len(indexes))

	for i, index := range indexes {
		columns := index.Columns()

		idx := &SpannerTableIndex{
			Name:    index.Name(),
			Columns: columns,
		}
		if unique, ok := index.Unique(); ok {
			idx.Unique = wrapperspb.Bool(unique)
		}

		res[i] = idx
	}

	return res, nil
}

// DeleteIndex deletes a Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that serves the index.
//   - indexName: string - Required. The name of the index to delete.
//
// Returns: *emptypb.Empty
func DeleteIndex(ctx context.Context, parent string, indexName string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.SpannerTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.SpannerTableNameRegex)
	}
	// Validate index name
	if valid := utils.ValidateArgument(indexName, utils.SpannerIndexIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index_name (%s), must match `%s`", indexName, utils.SpannerIndexIdRegex)
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
