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
		utils.SpannerGoogleSQLTableNameRegex,
		utils.SpannerPostgresSQLTableNameRegex,
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
		utils.SpannerGoogleSQLIndexIDRegex,
		utils.SpannerPostgresSQLIndexIDRegex,
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
			utils.SpannerGoogleSQLColumnIDRegex,
			utils.SpannerPostgresSQLColumnIDRegex,
		); err != nil {
			return nil, err
		}
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableID := parentName.Table

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return nil, err
	}

	// Create index
	ddl, err := index.CreateDdl(tableID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.conn.ExecuteDDL(ctx, database, ddl); err != nil {
		if isDuplicateNameInSchema(err, index.Name) {
			// Typically a redeploy after a timed-out apply whose CREATE INDEX
			// completed server-side. Adopting the index is only safe when it
			// is exactly what this create would have built.
			existing, getErr := s.GetSpannerTableIndex(ctx, parent, index.Name)
			if getErr == nil && indexesEquivalent(index, existing) {
				return existing, nil
			}
			return nil, status.Errorf(
				codes.AlreadyExists,
				"Index (%s) already exists on Table (%s) but does not match the planned definition; drop the existing index or align the configuration",
				index.Name,
				parent,
			)
		}
		return nil, status.Errorf(codes.Internal, "Error creating index: %v", err)
	}

	return index, nil
}

// indexesEquivalent reports whether an existing index is exactly the one a
// create would have built: same columns in the same sequence and sort order,
// same uniqueness. want holds configuration values (order may be UNSPECIFIED,
// unique may be nil — both meaning Spanner's defaults), got holds hydrated
// INFORMATION_SCHEMA values, so both sides are normalized before comparing.
func indexesEquivalent(want, got *SpannerTableIndex) bool {
	if want == nil || got == nil || len(want.Columns) != len(got.Columns) {
		return false
	}
	if want.Unique.GetValue() != got.Unique.GetValue() {
		return false
	}
	for i := range want.Columns {
		if want.Columns[i] == nil || got.Columns[i] == nil || want.Columns[i].Name != got.Columns[i].Name {
			return false
		}
		if normalizeIndexColumnOrder(want.Columns[i].Order) != normalizeIndexColumnOrder(got.Columns[i].Order) {
			return false
		}
	}
	return true
}

// normalizeIndexColumnOrder resolves UNSPECIFIED to ASC, mirroring CreateDdl.
func normalizeIndexColumnOrder(o SpannerTableIndexColumnOrder) SpannerTableIndexColumnOrder {
	if o == SpannerTableIndexColumnOrderUnspecified {
		return SpannerTableIndexColumnOrderAsc
	}
	return o
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
		utils.SpannerGoogleSQLTableNameRegex,
		utils.SpannerPostgresSQLTableNameRegex,
	); err != nil {
		return nil, err
	}
	if err := utils.ValidateDialectArgument(
		"name",
		name,
		utils.SpannerGoogleSQLIndexIDRegex,
		utils.SpannerPostgresSQLIndexIDRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableID := parentName.Table

	indexes, err := GetIndexes(ctx, s.conn, database, tableID)
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
		utils.SpannerGoogleSQLTableNameRegex,
		utils.SpannerPostgresSQLTableNameRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableID := parentName.Table

	indexes, err := GetIndexes(ctx, s.conn, database, tableID)
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
		utils.SpannerGoogleSQLTableNameRegex,
		utils.SpannerPostgresSQLTableNameRegex,
	); err != nil {
		return nil, err
	}
	if err := utils.ValidateDialectArgument(
		"index_name",
		indexName,
		utils.SpannerGoogleSQLIndexIDRegex,
		utils.SpannerPostgresSQLIndexIDRegex,
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
