package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/iam"
	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/utils"
)

// BigtableTable is a struct that represents a Bigtable table.
type BigtableTable struct {
	// Name is the full name of the table.
	// Format: projects/{project}/instances/{instance}/tables/{table}
	Name string
	// ColumnFamilies is a map of column family names to column families.
	ColumnFamilies map[string]bigtable.Family
	// DeletionProtection is the deletion protection setting for the table.
	DeletionProtection bigtable.DeletionProtection
	// ChangeStreamRetention is the change stream retention setting for the table.
	ChangeStreamRetention bigtable.ChangeStreamRetention
}

// BigtableBackup is a struct that represents a Bigtable backup.
type BigtableBackup struct {
	// Name is the full name of the backup.
	// Format: projects/{project}/instances/{instance}/clusters/{cluster}/backups/{backup}
	// [_a-zA-Z0-9][-_.a-zA-Z0-9]*{1,50}
	Name string
	// SourceTable is the full name of the table the backup was created from.
	// Format: projects/{project}/instances/{instance}/tables/{table}
	SourceTable string
	// SourceBackup is the full name of the backup from which this backup was copied. If a
	// backup is not created by copying a backup, this field will be empty.
	// Format: projects/{project}/instances/{instance}/backups/{backup}
	SourceBackup string
	// The size of the backup in bytes
	SizeBytes int64
	// The time the backup was started
	StartTime time.Time
	// The time the backup was finished
	EndTime time.Time
	// The expiration time of the backup, with microseconds
	// granularity that must be at least 6 hours and at most 90 days
	// from the time the request is received. Once the `expire_time`
	// has passed, Cloud Bigtable will delete the backup and free the
	// resources used by the backup.
	ExpireTime time.Time
	// The state of the backup
	State string
	// EncryptionInfo represents the encryption info of a backup.
	EncryptionInfo *bigtable.EncryptionInfo
}

// CreateBigtableTable creates a new Bigtable table with the given configuration.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The parent instance of the table.
//   - tableId: string - The ID of the table.
//   - table: *BigtableTable - The table configuration.
//
// Returns: *BigtableTable
func CreateBigtableTable(ctx context.Context, parent string, tableId string, table *BigtableTable) (*BigtableTable, error) {
	// Validate arguments
	// Validate parent name
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}
	// Validate Table Id
	if valid := utils.ValidateArgument(tableId, utils.BigtableTableIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table_id (%s), must match `%s`", tableId, utils.BigtableTableIdRegex)
	}
	// Ensure table is provided
	if table == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table, field is required but not provided")
	}
	// Validate column family names
	if table.ColumnFamilies != nil && len(table.ColumnFamilies) > 0 {
		for columnFamilyName := range table.ColumnFamilies {
			// Validate column family name
			if valid := utils.ValidateArgument(columnFamilyName, utils.BigtableColumnFamilyIdRegex); !valid {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.column_families.name (%s), must match `%s`", columnFamilyName, utils.BigtableColumnFamilyIdRegex)
			}
		}
	}

	// Deconstruct parent name to get project and instance
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instanceName := parentNameParts[3]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Set the table name
	table.Name = fmt.Sprintf("projects/%s/instances/%s/tables/%s", project, instanceName, tableId)

	// Create a new table with the given configuration
	err = client.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID:               tableId,
		SplitKeys:             nil,
		ColumnFamilies:        table.ColumnFamilies,
		DeletionProtection:    table.DeletionProtection,
		ChangeStreamRetention: table.ChangeStreamRetention,
	})
	if err != nil {
		return nil, err
	}

	// Return the table
	return table, nil
}

// GetBigtableTable gets the configuration of a Bigtable table.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - name: string - The name of the table.
//   - readMask: *fieldmaskpb.FieldMask - The set of fields to read.
//
// Returns: *BigtableTable
func GetBigtableTable(ctx context.Context, name string) (*BigtableTable, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(name, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s),  must match `%s`", name, utils.BigtableTableNameRegex)
	}

	// Get table ID from the name
	// Deconstruct name to get project and instance
	parentNameParts := strings.Split(name, "/")
	project := parentNameParts[1]
	instanceName := parentNameParts[3]
	tableID := parentNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Get table info
	tableInfo, err := client.TableInfo(ctx, tableID)
	if err != nil {
		// TODO: Handle not found error
		return nil, err
	}

	// Convert Bigtable column families to proto column families
	var columnFamilies map[string]bigtable.Family
	if tableInfo.FamilyInfos != nil && len(tableInfo.FamilyInfos) > 0 {
		columnFamilies = map[string]bigtable.Family{}

		// Iterate over the column families and convert them to proto column families
		for _, columnFamilyInfo := range tableInfo.FamilyInfos {
			// Add the column family to the map
			columnFamilies[columnFamilyInfo.Name] = bigtable.Family{
				GCPolicy: columnFamilyInfo.FullGCPolicy,
				// The type of data stored in each of this family's cell values, including its
				// full encoding. If omitted, the family only serves raw untyped bytes.
				ValueType: nil,
			}
		}
	}

	return &BigtableTable{
		Name:                  name,
		DeletionProtection:    tableInfo.DeletionProtection,
		ChangeStreamRetention: tableInfo.ChangeStreamRetention,
		ColumnFamilies:        columnFamilies,
	}, nil
}

// ListBigtableTables lists the Bigtable tables in an instance.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The parent instance of the tables.
//
// Returns: []*BigtableTable
func ListBigtableTables(ctx context.Context, parent string) ([]*BigtableTable, error) {
	// Validate parent
	if valid := utils.ValidateArgument(parent, utils.InstanceNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.InstanceNameRegex)
	}

	// Deconstruct parent name to get project and instance
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instanceName := parentNameParts[3]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Use the admin API to list the tables
	tableNames, err := client.Tables(ctx)
	if err != nil {
		return nil, err
	}

	// Get table information for each table and construct the response
	tables := make([]*BigtableTable, len(tableNames))

	// Iterate over the table names and get the table information
	for i, tableName := range tableNames {
		tableInfo, err := client.TableInfo(ctx, tableName)
		if err != nil {
			return nil, err
		}

		// Convert Bigtable column families to proto column families
		var columnFamilies map[string]bigtable.Family
		if tableInfo.FamilyInfos != nil && len(tableInfo.FamilyInfos) > 0 {
			columnFamilies = map[string]bigtable.Family{}

			// Iterate over the column families and convert them to proto column families
			for _, columnFamilyInfo := range tableInfo.FamilyInfos {
				// Add the column family to the map
				columnFamilies[columnFamilyInfo.Name] = bigtable.Family{
					GCPolicy: columnFamilyInfo.FullGCPolicy,
					// The type of data stored in each of this family's cell values, including its
					// full encoding. If omitted, the family only serves raw untyped bytes.
					ValueType: nil,
				}
			}
		}

		// Add the table to the response
		tables[i] = &BigtableTable{
			Name:                  fmt.Sprintf("projects/%s/instances/%s/tables/%s", os.Getenv("ALIS_OS_BT_PROJECT"), os.Getenv("ALIS_OS_BT_INSTANCE"), tableName),
			DeletionProtection:    tableInfo.DeletionProtection,
			ChangeStreamRetention: tableInfo.ChangeStreamRetention,
			ColumnFamilies:        columnFamilies,
		}
	}

	return tables, nil
}

// UpdateBigtableTable updates a Bigtable table with the given configuration.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - table: *BigtableTable - The table configuration.
//   - allowMissing: bool - Whether to allow missing table.
//
// Returns: *BigtableTable
func UpdateBigtableTable(ctx context.Context, table *BigtableTable, updateMask *fieldmaskpb.FieldMask, allowMissing bool) (*BigtableTable, error) {
	// Ensure table is provided
	if table == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table, field is required but not provided")
	}
	// Validate table name
	if valid := utils.ValidateArgument(table.Name, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.name (%s), must match `%s`", table.Name, utils.BigtableTableNameRegex)
	}
	// Validate column family names
	if table.ColumnFamilies != nil && len(table.ColumnFamilies) > 0 {
		for columnFamilyName := range table.ColumnFamilies {
			// Validate column family name
			if valid := utils.ValidateArgument(columnFamilyName, utils.BigtableColumnFamilyIdRegex); !valid {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.column_families.name (%s), must match `%s`", columnFamilyName, utils.BigtableColumnFamilyIdRegex)
			}
		}
	}
	// If update mask is not provided, ensure allow missing is set
	if updateMask == nil && !allowMissing {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument allow_missing, must be true if update_mask is not provided")
	}

	// Get existing table
	existingTable, err := GetBigtableTable(ctx, table.Name)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}
	// If table does not exist and allow missing is not set, return error
	if existingTable == nil && !allowMissing {
		return nil, status.Errorf(codes.NotFound, "Table %s not found, set allow_missing to true to create a new table", table.Name)
	}
	// If table exists, ensure update mask is provided
	if existingTable != nil && updateMask == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument update_mask, field is required but not provided")
	}

	// Deconstruct table name to get project, instance and table id
	tableNameParts := strings.Split(table.Name, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]
	tableID := tableNameParts[5]

	// If table does not exist, create the table
	if existingTable == nil {
		table, err := CreateBigtableTable(ctx, fmt.Sprintf("projects/%s/instances/%s", project, instanceName), tableID, table)
		if err != nil {
			return nil, err
		}

		return table, nil
	}

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Switch on update paths and update the necessary fields
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "deletion_protection":
			err := client.UpdateTableWithDeletionProtection(ctx, tableID, table.DeletionProtection)
			if err != nil {
				return nil, err
			}
		case "change_stream_retention":
			if table.ChangeStreamRetention == nil {
				// Disable change stream if retention is not provided
				err := client.UpdateTableDisableChangeStream(ctx, tableID)
				if err != nil {
					return nil, err
				}
			} else {
				// Update change stream retention
				err := client.UpdateTableWithChangeStream(ctx, tableID, table.ChangeStreamRetention)
				if err != nil {
					return nil, err
				}
			}
		case "column_families":
			// Compare column families and update as needed
			var removedColumnFamilies []string
			addedColumnFamilies := map[string]bigtable.Family{}

			// Get existing column families
			existingColumnFamilies := existingTable.ColumnFamilies
			newColumnFamilies := table.ColumnFamilies

			// If there's no existing column families, but new column families are provided, add all new column families
			if (existingColumnFamilies == nil || len(existingColumnFamilies) == 0) && (newColumnFamilies != nil && len(newColumnFamilies) > 0) {
				for columnFamilyName, columnFamily := range newColumnFamilies {
					addedColumnFamilies[columnFamilyName] = columnFamily
				}
			}

			// If there are existing column families, but no new column families are provided, remove all existing column families
			if (existingColumnFamilies != nil && len(existingColumnFamilies) > 0) && (newColumnFamilies == nil || len(newColumnFamilies) == 0) {
				for columnFamilyName := range existingColumnFamilies {
					removedColumnFamilies = append(removedColumnFamilies, columnFamilyName)
				}
			}

			// If there are existing column families and new column families are provided, compare and update
			if (existingColumnFamilies != nil && len(existingColumnFamilies) > 0) && (newColumnFamilies != nil && len(newColumnFamilies) > 0) {
				// Iterate over the existing column families and compare with the new column families
				for existingColumnFamilyName := range existingColumnFamilies {
					if _, exists := newColumnFamilies[existingColumnFamilyName]; !exists {
						// Column family does not exist in new column families, remove it
						removedColumnFamilies = append(removedColumnFamilies, existingColumnFamilyName)
					}
				}

				// Iterate over the new column families and compare with the existing column families
				for newColumnFamilyName, newColumnFamily := range newColumnFamilies {
					if _, exists := existingColumnFamilies[newColumnFamilyName]; !exists {
						// Column family does not exist in existing column families, add it
						addedColumnFamilies[newColumnFamilyName] = newColumnFamily
					}
				}
			}

			if len(removedColumnFamilies) > 0 {
				// Iterate over the removed column families and remove them
				for _, removedColumnFamily := range removedColumnFamilies {
					err := client.DeleteColumnFamily(ctx, tableID, removedColumnFamily)
					if err != nil {
						return nil, err
					}
				}
			}

			if len(addedColumnFamilies) > 0 {
				// Iterate over the added column families and add them
				for addedColumnFamilyName, addedColumnFamily := range addedColumnFamilies {
					// Create column family
					err := client.CreateColumnFamilyWithConfig(ctx, tableID, addedColumnFamilyName, addedColumnFamily)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return table, nil
}

// DeleteBigtableTable deletes a Bigtable table.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - name: string - The name of the table.
//
// Returns: *emptypb.Empty
func DeleteBigtableTable(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate table name
	if valid := utils.ValidateArgument(name, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.BigtableTableNameRegex)
	}

	// Deconstruct name to get project and instance
	parentNameParts := strings.Split(name, "/")
	project := parentNameParts[1]
	instanceName := parentNameParts[3]
	tableID := parentNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Use the admin API to delete the table
	err = client.DeleteTable(ctx, tableID)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// GetBigtableTableIamPolicy gets the IAM policy for a Bigtable table.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//   - options: *iampb.GetPolicyOptions - The options for getting the policy.
//
// Returns: *iampb.Policy
func GetBigtableTableIamPolicy(ctx context.Context, parent string, options *iampb.GetPolicyOptions) (*iampb.Policy, error) {
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}

	// Deconstruct table name to get project, instance and table id
	tableNameParts := strings.Split(parent, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]
	tableID := tableNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	iamHandle := client.TableIAM(tableID)

	if options != nil && options.GetRequestedPolicyVersion() == 3 {
		policy, err := iamHandle.V3().Policy(ctx)
		if err != nil {
			return nil, err
		}

		return &iampb.Policy{
			Version:  options.GetRequestedPolicyVersion(),
			Bindings: policy.Bindings,
		}, nil
	}

	// Get the IAM policy
	policy, err := iamHandle.Policy(ctx)
	if err != nil {
		return nil, err
	}

	return policy.InternalProto, nil
}

// SetBigtableTableIamPolicy sets the IAM policy for a Bigtable table.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//   - policy: *iampb.Policy - The new IAM policy.
//   - updateMask: *fieldmaskpb.FieldMask - The set of fields to update.
//
// Returns: *iampb.Policy
func SetBigtableTableIamPolicy(ctx context.Context, parent string, policy *iampb.Policy, updateMask *fieldmaskpb.FieldMask) (*iampb.Policy, error) {
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}
	// Ensure policy is provided
	if policy == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument policy, field is required but not provided")
	}
	// If update mask is provided, validate it
	if updateMask != nil && len(updateMask.GetPaths()) > 0 {
		// Normalize the update mask
		updateMask.Normalize()
		if valid := updateMask.IsValid(&iampb.Policy{}); !valid {
			return nil, status.Error(codes.InvalidArgument, "invalid update mask")
		}
	}

	// Deconstruct table name to get project, instance and table id
	tableNameParts := strings.Split(parent, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]
	tableID := tableNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// If the policy version is 3, use the V3 IAM handle
	if policy.GetVersion() == 3 {
		// Get the IAM handle
		iamHandle := client.TableIAM(tableID).V3()

		// Create a new policy from request
		err := iamHandle.SetPolicy(ctx, &iam.Policy3{
			Bindings: policy.GetBindings(),
		})
		if err != nil {
			return nil, err
		}

		return policy, nil
	}

	// Create a new policy from request
	newPolicy := &iam.Policy{}
	for _, binding := range policy.GetBindings() {
		for _, member := range binding.GetMembers() {
			newPolicy.Add(member, iam.RoleName(binding.GetRole()))
		}
	}

	// Get the IAM handle
	iamHandle := client.TableIAM(tableID)

	// Set the IAM policy
	err = iamHandle.SetPolicy(ctx, newPolicy)
	if err != nil {
		return nil, err
	}

	return newPolicy.InternalProto, nil
}

// CreateBigtableColumnFamily creates a new Bigtable column family with the given configuration.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The parent table of the column family.
//   - columnFamilyId: string - The ID of the column family.
//   - columnFamily: *bigtable.Family - The column family configuration.
//
// Returns: *bigtable.Family
func CreateBigtableColumnFamily(ctx context.Context, parent string, columnFamilyId string, columnFamily *bigtable.Family) (*bigtable.Family, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}
	// Validate column family id
	if valid := utils.ValidateArgument(columnFamilyId, utils.BigtableColumnFamilyIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument column_family_id (%s), must match `%s`", columnFamilyId, utils.BigtableColumnFamilyIdRegex)
	}
	// Ensure column family is provided
	if columnFamily == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument column_family, field is required but not provided")
	}

	// Deconstruct table name to get project, instance and table id
	tableNameParts := strings.Split(parent, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	err = client.CreateColumnFamilyWithConfig(ctx, parent, columnFamilyId, *columnFamily)
	if err != nil {
		return nil, err
	}

	return columnFamily, nil
}

// ListBigtableColumnFamilies lists the column families of a Bigtable table.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//
// Returns: []*bigtable.Family
func ListBigtableColumnFamilies(ctx context.Context, parent string) (map[string]*bigtable.Family, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}

	// Get table
	table, err := GetBigtableTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	res := map[string]*bigtable.Family{}

	// Iterate over the column families and add them to the response
	if table.ColumnFamilies != nil && len(table.ColumnFamilies) > 0 {
		for columnFamilyName, columnFamily := range table.ColumnFamilies {
			res[columnFamilyName] = &columnFamily
		}
	}

	return res, nil
}

// DeleteBigtableColumnFamily deletes a Bigtable column family.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//   - columnFamilyId: string - The ID of the column family.
//
// Returns: *emptypb.Empty
func DeleteBigtableColumnFamily(ctx context.Context, parent string, columnFamilyId string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}
	// Validate column family id
	if valid := utils.ValidateArgument(columnFamilyId, utils.BigtableColumnFamilyIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument column_family_id (%s), must match `%s`", columnFamilyId, utils.BigtableColumnFamilyIdRegex)
	}

	// Deconstruct table name to get project, instance and table id
	tableNameParts := strings.Split(parent, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Get table
	_, err = GetBigtableTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Delete column family
	err = client.DeleteColumnFamily(ctx, parent, columnFamilyId)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// GetBigtableGarbageCollectionPolicy gets the garbage collection policy for a Bigtable column family.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//   - columnFamilyId: string - The ID of the column family.
//
// Returns: *bigtable.GCPolicy
func GetBigtableGarbageCollectionPolicy(ctx context.Context, parent string, columnFamilyId string) (*bigtable.GCPolicy, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}
	// Validate column family id
	if valid := utils.ValidateArgument(columnFamilyId, utils.BigtableColumnFamilyIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument column_family_id (%s), must match `%s`", columnFamilyId, utils.BigtableColumnFamilyIdRegex)
	}

	// Get table
	table, err := GetBigtableTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Get column family
	if table.ColumnFamilies == nil || len(table.ColumnFamilies) == 0 {
		return nil, status.Errorf(codes.NotFound, "Column family %s not found", columnFamilyId)
	}

	// Get the column family
	cf, exists := table.ColumnFamilies[columnFamilyId]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Column family %s not found", columnFamilyId)
	}
	if cf.GCPolicy == nil {
		return nil, status.Errorf(codes.NotFound, "Column family %s does not have a GC policy", columnFamilyId)
	}

	gcPolicy := table.ColumnFamilies[columnFamilyId].GCPolicy

	return &gcPolicy, nil
}

// UpdateBigtableGarbageCollectionPolicy updates the garbage collection policy for a Bigtable column family.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//   - columnFamilyId: string - The ID of the column family.
//   - gcPolicy: *bigtable.GCPolicy - The new garbage collection policy.
//
// Returns: *bigtable.GCPolicy
func UpdateBigtableGarbageCollectionPolicy(ctx context.Context, parent string, columnFamilyId string, gcPolicy *bigtable.GCPolicy) (*bigtable.GCPolicy, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}
	// Validate column family id
	if valid := utils.ValidateArgument(columnFamilyId, utils.BigtableColumnFamilyIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument column_family_id (%s), must match `%s`", columnFamilyId, utils.BigtableColumnFamilyIdRegex)
	}

	// Get table
	table, err := GetBigtableTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// TODO: Implement allow_missing/recreate policy

	// Convert GarbageCollectionPolicy_GcRule to bigtable.GCPolicy
	newPolicy := bigtable.NoGcPolicy()
	if gcPolicy != nil {
		newPolicy = *gcPolicy
	}

	// Deconstruct table name to get table id
	tableNameParts := strings.Split(table.Name, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]
	tableID := tableNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Set the column family GC policy
	err = client.SetGCPolicy(ctx, tableID, columnFamilyId, newPolicy)
	if err != nil {
		return nil, err
	}

	return &newPolicy, nil
}

// ListBigtableGarbageCollectionPolicies lists the garbage collection policies for a Bigtable table.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//
// Returns: []*bigtable.GCPolicy
func ListBigtableGarbageCollectionPolicies(ctx context.Context, parent string) ([]*bigtable.GCPolicy, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}

	// Get table
	table, err := GetBigtableTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	var res []*bigtable.GCPolicy

	// Get column family
	if table.ColumnFamilies == nil || len(table.ColumnFamilies) == 0 {
		return res, nil
	}

	// Iterate over the column families and add them to the response
	for _, cf := range table.ColumnFamilies {
		if cf.GCPolicy == nil {
			continue
		}

		res = append(res, &cf.GCPolicy)
	}

	return res, nil
}

// DeleteBigtableGarbageCollectionPolicy deletes the garbage collection policy for a Bigtable column family.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the table.
//   - columnFamilyId: string - The ID of the column family.
//
// Returns: *emptypb.Empty
func DeleteBigtableGarbageCollectionPolicy(ctx context.Context, parent string, columnFamilyId string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate table name
	if valid := utils.ValidateArgument(parent, utils.BigtableTableNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableTableNameRegex)
	}
	// Validate column family id
	if valid := utils.ValidateArgument(columnFamilyId, utils.BigtableColumnFamilyIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument column_family_id (%s), must match `%s`", columnFamilyId, utils.BigtableColumnFamilyIdRegex)
	}

	// Get table
	table, err := GetBigtableTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Deconstruct table name to get table id
	tableNameParts := strings.Split(table.Name, "/")
	project := tableNameParts[1]
	instanceName := tableNameParts[3]
	tableID := tableNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Set the column family GC policy to NoGcPolicy
	err = client.SetGCPolicy(ctx, tableID, columnFamilyId, bigtable.NoGcPolicy())
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// CreateBigtableBackup creates a new Bigtable backup with the given configuration.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The parent cluster of the backup.
//   - backupId: string - The ID of the backup.
//   - backup: *BigtableBackup - The backup configuration.
//
// Returns: *BigtableBackup
func CreateBigtableBackup(ctx context.Context, parent string, backupId string, backup *BigtableBackup) (*BigtableBackup, error) {
	// Validate arguments
	// Validate parent name
	if valid := utils.ValidateArgument(parent, utils.BigtableClusterNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableClusterNameRegex)
	}
	// Validate backup id
	if valid := utils.ValidateArgument(backupId, utils.BigtableBackupIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup_id (%s), must match `%s`", backupId, utils.BigtableBackupIdRegex)
	}
	// Ensure backup is provided
	if backup == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup, field is required but not provided")
	}
	// Validate table id
	if valid := utils.ValidateArgument(backup.SourceTable, utils.BigtableTableIdRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup.source_table (%s), must match `%s`", backup.SourceTable, utils.BigtableTableIdRegex)
	}

	// Deconstruct parent name to get project, instance and cluster id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instanceName := parentNameParts[3]
	cluster := parentNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Set the backup name
	backup.Name = fmt.Sprintf("projects/%s/instances/%s/clusters/%s/backups/%s", project, instanceName, cluster, backupId)

	// Get the backup
	existingBackup, err := GetBigtableBackup(ctx, backup.Name)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}
	// If backup exists, return error
	if existingBackup != nil {
		return nil, status.Errorf(codes.AlreadyExists, "Backup %s already exists", backup.Name)
	}

	// Create the backup
	err = client.CreateBackup(ctx, backup.SourceTable, cluster, backupId, backup.ExpireTime)
	if err != nil {
		return nil, err
	}

	// Get the updated backup
	updatedBackup, err := GetBigtableBackup(ctx, backup.Name)
	if err != nil {
		return nil, err
	}

	return updatedBackup, nil
}

// GetBigtableBackup gets a Bigtable backup.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - name: string - The name of the backup.
//
// Returns: *BigtableBackup
func GetBigtableBackup(ctx context.Context, name string) (*BigtableBackup, error) {
	// Validate arguments
	// Validate backup name
	if valid := utils.ValidateArgument(name, utils.BigtableBackupNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.BigtableBackupNameRegex)
	}

	// Deconstruct backup name to get project, instance, cluster and backup id
	backupNameParts := strings.Split(name, "/")
	project := backupNameParts[1]
	instanceName := backupNameParts[3]
	cluster := backupNameParts[5]
	backupId := backupNameParts[7]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Get backup info
	backupInfo, err := client.BackupInfo(ctx, cluster, backupId)
	if err != nil {
		return nil, err
	}

	return &BigtableBackup{
		Name:           name,
		SourceTable:    backupInfo.SourceTable,
		SourceBackup:   backupInfo.SourceBackup,
		SizeBytes:      backupInfo.SizeBytes,
		StartTime:      backupInfo.StartTime,
		EndTime:        backupInfo.EndTime,
		ExpireTime:     backupInfo.ExpireTime,
		State:          backupInfo.State,
		EncryptionInfo: backupInfo.EncryptionInfo,
	}, nil
}

// ListBigtableBackups lists the Bigtable backups in a cluster.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - parent: string - The name of the cluster.
//   - pageSize: int32 - The maximum number of backups to return.
//   - pageToken: string - The page token to resume from.
//
// Returns: []*BigtableBackup, string
func ListBigtableBackups(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*BigtableBackup, string, error) {
	// Validate arguments
	// Validate parent name
	if valid := utils.ValidateArgument(parent, utils.BigtableClusterNameRegex); !valid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s`", parent, utils.BigtableClusterNameRegex)
	}

	// Deconstruct parent name to get project, instance and cluster id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instanceName := parentNameParts[3]
	cluster := parentNameParts[5]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	var res []*BigtableBackup
	var nextPageToken string

	// Get backup names
	it := client.Backups(ctx, cluster)
	for {
		backupInfo, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		res = append(res, &BigtableBackup{
			Name:           fmt.Sprintf("projects/%s/instances/%s/clusters/%s/backups/%s", project, instanceName, cluster, backupInfo.Name),
			SourceTable:    backupInfo.SourceTable,
			SourceBackup:   backupInfo.SourceBackup,
			SizeBytes:      backupInfo.SizeBytes,
			StartTime:      backupInfo.StartTime,
			EndTime:        backupInfo.EndTime,
			ExpireTime:     backupInfo.ExpireTime,
			State:          backupInfo.State,
			EncryptionInfo: backupInfo.EncryptionInfo,
		})

		// Check if page size is reached
		if pageSize > 0 && len(res) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
	}

	return res, nextPageToken, nil
}

// UpdateBigtableBackup updates a Bigtable backup.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - backup: *BigtableBackup - The updated backup.
//   - updateMask: *fieldmaskpb.FieldMask - The set of fields to update.
//   - allowMissing: bool - If set to true, allows the creation of a new backup if the backup does not exist.
//
// Returns: *BigtableBackup
func UpdateBigtableBackup(ctx context.Context, backup *BigtableBackup, updateMask *fieldmaskpb.FieldMask, allowMissing bool) (*BigtableBackup, error) {
	// Validate arguments
	// Ensure backup is provided
	if backup == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup, field is required but not provided")
	}
	// Validate backup name
	if valid := utils.ValidateArgument(backup.Name, utils.BigtableBackupNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument backup.name (%s), must match `%s`", backup.Name, utils.BigtableBackupNameRegex)
	}
	// If update mask is not provided, ensure allow missing is set
	if updateMask == nil && !allowMissing {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument allow_missing, must be true if update_mask is not provided")
	}

	// Get backup
	existingBackup, err := GetBigtableBackup(ctx, backup.Name)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}
	// If backup does not exist, return error
	if existingBackup == nil && !allowMissing {
		return nil, status.Errorf(codes.NotFound, "Backup %s not found, set allow_missing to true to create a new backup", backup.Name)
	}
	// If backup exists, ensure update mask is provided
	if existingBackup != nil && updateMask == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument update_mask, field is required but not provided")
	}

	// Deconstruct backup name to get project, instance, cluster and backup id
	backupNameParts := strings.Split(backup.Name, "/")
	project := backupNameParts[1]
	instanceName := backupNameParts[3]
	cluster := backupNameParts[5]
	backupId := backupNameParts[7]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// If backup is not found and allow missing is set, create the backup
	if existingBackup == nil {
		newBackup, err := CreateBigtableBackup(ctx, fmt.Sprintf("projects/%s/instances/%s/clusters/%s", project, instanceName, cluster), backupId, backup)
		if err != nil {
			return nil, err
		}

		return newBackup, nil
	}

	// Switch on update paths and update the necessary fields
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "expire_time":
			// Update expire time
			err := client.UpdateBackup(ctx, cluster, backupId, backup.ExpireTime)
			if err != nil {
				return nil, err
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument update_mask, only expire_time not supported")
		}
	}

	// Get the updated backup
	updatedBackup, err := GetBigtableBackup(ctx, backup.Name)
	if err != nil {
		return nil, err
	}

	return updatedBackup, nil
}

// DeleteBigtableBackup deletes a Bigtable backup.
//
// Params:
//   - ctx: context.Context - The context to use for the RPCs.
//   - name: string - The name of the backup.
//
// Returns: *emptypb.Empty
func DeleteBigtableBackup(ctx context.Context, name string) (*emptypb.Empty, error) {
	// Validate arguments
	// Validate backup name
	if valid := utils.ValidateArgument(name, utils.BigtableBackupNameRegex); !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s`", name, utils.BigtableBackupNameRegex)
	}

	// Deconstruct backup name to get project, instance, cluster and backup id
	backupNameParts := strings.Split(name, "/")
	project := backupNameParts[1]
	instanceName := backupNameParts[3]
	cluster := backupNameParts[5]
	backupId := backupNameParts[7]

	client, err := bigtable.NewAdminClient(context.Background(), project, instanceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create Bigtable admin client")
	}

	// Delete the backup
	err = client.DeleteBackup(ctx, cluster, backupId)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
