package schema

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// SpannerTableRowDeletionPolicy represents a table's TTL policy: rows are
// deleted once Column is older than Duration days.
type SpannerTableRowDeletionPolicy struct {
	// The name of the TIMESTAMP column that is used to determine when a row is deleted
	Column string
	// The duration after which a row is deleted in days
	Duration *wrapperspb.Int64Value
}

func (p *SpannerTableRowDeletionPolicy) ddl(table, verb string) (string, error) {
	if p == nil {
		return "", nil
	}
	if table == "" {
		return "", errors.New("table is required for row deletion policy")
	}
	if p.Column == "" {
		return "", errors.New("column is required for row deletion policy")
	}
	if p.Duration == nil {
		return "", errors.New("duration is required for row deletion policy")
	}

	return fmt.Sprintf("ALTER TABLE %s %s ROW DELETION POLICY (OLDER_THAN(%s, INTERVAL %d DAY))",
		table, verb, p.Column, p.Duration.GetValue()), nil
}

// CreateDdl renders the ADD ROW DELETION POLICY statement.
func (p *SpannerTableRowDeletionPolicy) CreateDdl(table string) (string, error) {
	return p.ddl(table, "ADD")
}

// ReplaceDdl renders the REPLACE ROW DELETION POLICY statement.
func (p *SpannerTableRowDeletionPolicy) ReplaceDdl(table string) (string, error) {
	return p.ddl(table, "REPLACE")
}

// DropRowDeletionPolicyDdl renders the DROP ROW DELETION POLICY statement.
func DropRowDeletionPolicyDdl(table string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP ROW DELETION POLICY", table)
}
