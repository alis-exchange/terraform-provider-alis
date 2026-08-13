package services

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"
)

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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	sqlStatement := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (`%s`) REFERENCES %s(`%s`)", tableId, constraint.Name, constraint.Column, constraint.ReferencedTable, constraint.ReferencedColumn)
	if constraint.OnDelete != schema.SpannerTableConstraintActionUnspecified {
		sqlStatement += fmt.Sprintf(" ON DELETE %s", constraint.OnDelete.String())
	}
	if err := s.conn.ExecuteDDL(ctx, database, sqlStatement); err != nil {
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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

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

	var result Constraint
	if err := s.conn.Query(ctx, database, &result, sqlStatement, tableId, name); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "Foreign key constraint %s not found", name)
		}
		return nil, status.Errorf(codes.Internal, "Error getting foreign key constraint: %v", err)
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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	if err := s.conn.ExecuteDDL(ctx, database, fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", tableId, name)); err != nil {
		return status.Errorf(codes.Internal, "Error dropping foreign key constraint: %v", err)
	}

	return nil
}
