package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

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

		// If column type is PROTO ensure the proto package is provided
		if column.Type == schema.SpannerTableDataTypeProto.String() && column.GetProtoPackage().GetValue() == "" {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_package, field is required but not provided", i)
		}
	}

	// Set table name
	table.Name = fmt.Sprintf("%s/tables/%s", parent, tableId)

	if _, err := names.ParseDatabase(parent); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}

	// Create table
	_, err := utils.Retry(3, 5*time.Second, func() (interface{}, error) {
		_, err := table.Create(ctx, s.conn)
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

	table, err := (&schema.SpannerTable{}).Get(ctx, s.conn, name)
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
//   - updateMask: *fieldmaskpb.FieldMask - The fields to update. Only `schema.columns` is supported; required when the table already exists.
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

					// If column type is PROTO ensure the proto package is provided
					if column.GetType() == schema.SpannerTableDataTypeProto.String() && column.GetProtoPackage().GetValue() == "" {
						return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.schema.columns[%d].proto_package, field is required but not provided", i)
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

	tableName, err := names.ParseTable(table.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument table.name (%s): %v", table.Name, err)
	}
	tableId := tableName.Table

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
		return s.CreateSpannerTable(ctx, tableName.DatabaseName().String(), tableId, table)
	}

	_, err = table.Update(ctx, s.conn, existingTable)
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

	err = table.Delete(ctx, s.conn)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
