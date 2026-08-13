package services

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateSpannerTableIndex creates a new Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that will serve the new index.
//   - index: *SpannerTableIndex - Required. The index to create.
//
// Returns: *SpannerTableIndex.
func (s *SpannerService) CreateSpannerTableIndex(ctx context.Context, parent string, index *SpannerTableIndex) (*SpannerTableIndex, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}
	// Ensure index is provided and has a name and columns
	if index == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument index, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"index.name",
		index.Name,
		utils.SpannerGoogleSqlIndexIdRegex,
		utils.SpannerPostgresSqlIndexIdRegex,
	); err != nil {
		return nil, err
	}
	if len(index.Columns) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument index.columns, field is required but not provided")
	}
	for i, column := range index.Columns {
		if column == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid argument index.columns[%d], field is required but not provided", i)
		}

		if err := utils.ValidateDialectArgument(
			"index.columns[%d].name",
			column.Name,
			utils.SpannerGoogleSqlColumnIdRegex,
			utils.SpannerPostgresSqlColumnIdRegex,
		); err != nil {
			return nil, err
		}
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return nil, err
	}

	// Create index
	ddl, err := index.CreateDdl(tableId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.conn.ExecuteDDL(ctx, database, ddl); err != nil {
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
// Returns: *SpannerTableIndex.
func (s *SpannerService) GetSpannerTableIndex(ctx context.Context, parent, name string) (*SpannerTableIndex, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}
	if err := utils.ValidateDialectArgument(
		"name",
		name,
		utils.SpannerGoogleSqlIndexIdRegex,
		utils.SpannerPostgresSqlIndexIdRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	indexes, err := GetIndexes(ctx, s.conn, database, tableId)
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
// Returns: []*SpannerTableIndex.
func (s *SpannerService) ListSpannerTableIndices(ctx context.Context, parent string) ([]*SpannerTableIndex, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	indexes, err := GetIndexes(ctx, s.conn, database, tableId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table indices: %v", err)
	}

	return indexes, nil
}

// DeleteSpannerTableIndex deletes a Spanner table index.
//
// Params:
//   - ctx: context.Context - The context to use for RPCs.
//   - parent: string - Required. The name of the table that serves the index.
//   - indexName: string - Required. The name of the index to delete.
//
// Returns: *emptypb.Empty.
func (s *SpannerService) DeleteSpannerTableIndex(ctx context.Context, parent, indexName string) (*emptypb.Empty, error) {
	// Validate arguments
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}
	if err := utils.ValidateDialectArgument(
		"index_name",
		indexName,
		utils.SpannerGoogleSqlIndexIdRegex,
		utils.SpannerPostgresSqlIndexIdRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()

	// Drop the index
	if err := s.conn.ExecuteDDL(ctx, database, schema.DropIndexDdl(indexName)); err != nil {
		return nil, status.Errorf(codes.Internal, "Error dropping index: %v", err)
	}

	return &emptypb.Empty{}, nil
}
