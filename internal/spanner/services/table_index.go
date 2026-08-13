package services

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-alis/internal/utils"

	spannergorm "github.com/googleapis/go-gorm-spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

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
