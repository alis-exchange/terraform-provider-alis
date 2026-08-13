package schema

import (
	"errors"
	"fmt"
)

// SpannerTableForeignKeyConstraint represents a single-column foreign key:
// Column references ReferencedColumn on ReferencedTable.
type SpannerTableForeignKeyConstraint struct {
	// The name of the constraint
	Name string
	// Referenced table
	ReferencedTable string
	// Referenced column
	ReferencedColumn string
	// Referencing column
	Column string
	// Referential actions on delete
	OnDelete SpannerTableConstraintAction
}

// SpannerTableConstraintAction is the referential action a constraint or
// interleave applies ON DELETE.
type SpannerTableConstraintAction int64

const (
	SpannerTableConstraintActionUnspecified SpannerTableConstraintAction = iota
	SpannerTableConstraintActionCascade
	SpannerTableConstraintNoAction
)

// String returns the DDL keyword for the action; Unspecified renders as "".
func (a SpannerTableConstraintAction) String() string {
	return [...]string{"", "CASCADE", "NO ACTION"}[a]
}

// SpannerTableConstraintActionFromString parses a DDL keyword ("CASCADE",
// "NO ACTION"), returning Unspecified for anything else.
func SpannerTableConstraintActionFromString(s string) SpannerTableConstraintAction {
	switch s {
	case "CASCADE":
		return SpannerTableConstraintActionCascade
	case "NO ACTION":
		return SpannerTableConstraintNoAction
	default:
		return SpannerTableConstraintActionUnspecified
	}
}

// SpannerTableConstraintActions lists the actions accepted in configuration
// (Unspecified is excluded).
var SpannerTableConstraintActions = []string{
	SpannerTableConstraintActionCascade.String(),
	SpannerTableConstraintNoAction.String(),
}

// CreateDdl renders the ADD CONSTRAINT ... FOREIGN KEY statement for the
// constraint on the given table, with an ON DELETE clause when an action is set.
func (c *SpannerTableForeignKeyConstraint) CreateDdl(table string) (string, error) {
	if c == nil {
		return "", nil
	}
	if table == "" {
		return "", fmt.Errorf("table is required for foreign key constraint %s", c.Name)
	}
	if c.Name == "" {
		return "", errors.New("constraint name is required")
	}
	if c.Column == "" {
		return "", fmt.Errorf("column is required for foreign key constraint %s", c.Name)
	}
	if c.ReferencedTable == "" {
		return "", fmt.Errorf("referenced table is required for foreign key constraint %s", c.Name)
	}
	if c.ReferencedColumn == "" {
		return "", fmt.Errorf("referenced column is required for foreign key constraint %s", c.Name)
	}

	ddl := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (`%s`) REFERENCES %s(`%s`)",
		table, c.Name, c.Column, c.ReferencedTable, c.ReferencedColumn)
	if c.OnDelete != SpannerTableConstraintActionUnspecified {
		ddl += " ON DELETE " + c.OnDelete.String()
	}
	return ddl, nil
}

// DropForeignKeyConstraintDdl renders the DROP CONSTRAINT statement.
func DropForeignKeyConstraintDdl(table, name string) string {
	return fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", table, name)
}
