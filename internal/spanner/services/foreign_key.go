package services

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateSpannerTableForeignKeyConstraint adds a foreign key constraint to the
// table named by parent via ALTER TABLE ... ADD CONSTRAINT. Every constraint
// field is validated up front; DDL failures surface as codes.Internal.
func (s *SpannerService) CreateSpannerTableForeignKeyConstraint(
	ctx context.Context,
	parent string,
	constraint *schema.SpannerTableForeignKeyConstraint,
) (*schema.SpannerTableForeignKeyConstraint, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSQLTableNameRegex,
		utils.SpannerPostgresSQLTableNameRegex,
	); err != nil {
		return nil, err
	}
	// Ensure constraint is provided and has a name and foreign keys
	if constraint == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"constraint.name",
		constraint.Name,
		utils.SpannerGoogleSQLConstraintIDRegex,
		utils.SpannerPostgresSQLConstraintIDRegex,
	); err != nil {
		return nil, err
	}

	if constraint.ReferencedTable == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint.referenced_table, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"constraint.referenced_table",
		constraint.ReferencedTable,
		utils.SpannerGoogleSQLTableIDRegex,
		utils.SpannerPostgresSQLTableIDRegex,
	); err != nil {
		return nil, err
	}

	if constraint.ReferencedColumn == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint.referenced_column, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"constraint.referenced_column",
		constraint.ReferencedColumn,
		utils.SpannerGoogleSQLColumnIDRegex,
		utils.SpannerPostgresSQLColumnIDRegex,
	); err != nil {
		return nil, err
	}

	if constraint.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument constraint.column, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"constraint.column",
		constraint.Column,
		utils.SpannerGoogleSQLColumnIDRegex,
		utils.SpannerPostgresSQLColumnIDRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableID := parentName.Table

	ddl, err := constraint.CreateDdl(tableID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.conn.ExecuteDDL(ctx, database, ddl); err != nil {
		if isDuplicateNameInSchema(err, constraint.Name) {
			// Typically a redeploy after a timed-out apply whose ADD CONSTRAINT
			// completed server-side. Adopting the constraint is only safe when
			// it is exactly what this create would have built. The lookup is
			// scoped to parent, so a same-named constraint on another table
			// stays a conflict.
			existing, getErr := s.GetSpannerTableForeignKeyConstraint(ctx, parent, constraint.Name)
			if getErr == nil && foreignKeyConstraintsEquivalent(constraint, existing) {
				return existing, nil
			}
			return nil, status.Errorf(
				codes.AlreadyExists,
				"Foreign Key Constraint (%s) already exists in the schema but does not match the planned definition on Table (%s); drop the existing constraint or align the configuration",
				constraint.Name,
				parent,
			)
		}
		return nil, status.Errorf(codes.Internal, "Error creating foreign key constraint: %v", err)
	}

	return constraint, nil
}

// foreignKeyConstraintsEquivalent reports whether an existing constraint is
// exactly the one a create would have built. want holds configuration values
// (OnDelete may be Unspecified, which Spanner materializes as NO ACTION), got
// holds hydrated INFORMATION_SCHEMA values, so OnDelete is normalized before
// comparing.
func foreignKeyConstraintsEquivalent(want, got *schema.SpannerTableForeignKeyConstraint) bool {
	if want == nil || got == nil {
		return false
	}
	normalize := func(a schema.SpannerTableConstraintAction) schema.SpannerTableConstraintAction {
		if a == schema.SpannerTableConstraintActionUnspecified {
			return schema.SpannerTableConstraintNoAction
		}
		return a
	}
	return want.Column == got.Column &&
		want.ReferencedTable == got.ReferencedTable &&
		want.ReferencedColumn == got.ReferencedColumn &&
		normalize(want.OnDelete) == normalize(got.OnDelete)
}

// GetSpannerTableForeignKeyConstraint reconstructs a foreign key constraint
// from the INFORMATION_SCHEMA constraint tables. parent is the constrained
// table's resource name and name the bare constraint ID; codes.NotFound is
// returned when the table has no FOREIGN KEY constraint by that name.
func (s *SpannerService) GetSpannerTableForeignKeyConstraint(
	ctx context.Context,
	parent, name string,
) (*schema.SpannerTableForeignKeyConstraint, error) {
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
		utils.SpannerGoogleSQLConstraintIDRegex,
		utils.SpannerPostgresSQLConstraintIDRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableID := parentName.Table

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
	if err := s.conn.Query(ctx, database, &result, sqlStatement, tableID, name); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "Foreign key constraint %s not found", name)
		}
		return nil, status.Errorf(codes.Internal, "Error getting foreign key constraint: %v", err)
	}

	constaint := &schema.SpannerTableForeignKeyConstraint{
		Name:             result.ConstraintName,
		ReferencedTable:  result.ReferencedTable,
		ReferencedColumn: result.ReferencedColumn,
		Column:           result.ConstrainedColumn,
		OnDelete:         schema.SpannerTableConstraintActionFromString(result.DeleteRule),
	}

	return constaint, nil
}

// DeleteSpannerTableForeignKeyConstraint drops the named foreign key
// constraint from the table via ALTER TABLE ... DROP CONSTRAINT.
func (s *SpannerService) DeleteSpannerTableForeignKeyConstraint(ctx context.Context, parent, name string) error {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSQLTableNameRegex,
		utils.SpannerPostgresSQLTableNameRegex,
	); err != nil {
		return err
	}

	if err := utils.ValidateDialectArgument(
		"name",
		name,
		utils.SpannerGoogleSQLConstraintIDRegex,
		utils.SpannerPostgresSQLConstraintIDRegex,
	); err != nil {
		return err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableID := parentName.Table

	if err := s.conn.ExecuteDDL(ctx, database, schema.DropForeignKeyConstraintDdl(tableID, name)); err != nil {
		return status.Errorf(codes.Internal, "Error dropping foreign key constraint: %v", err)
	}

	return nil
}
