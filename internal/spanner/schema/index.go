package schema

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// SpannerTableIndexColumnOrder is the sort order of a column within an index.
type SpannerTableIndexColumnOrder int64

const (
	SpannerTableIndexColumnOrderUnspecified SpannerTableIndexColumnOrder = iota
	SpannerTableIndexColumnOrderAsc
	SpannerTableIndexColumnOrderDesc
)

// String returns the lowercase configuration spelling; CreateDdl upper-cases
// it for DDL.
func (s SpannerTableIndexColumnOrder) String() string {
	return [...]string{"unspecified", "asc", "desc"}[s]
}

// SpannerTableIndexColumnOrders lists the orders accepted in configuration
// (UNSPECIFIED is excluded).
var SpannerTableIndexColumnOrders = []string{
	SpannerTableIndexColumnOrderAsc.String(),
	SpannerTableIndexColumnOrderDesc.String(),
}

// SpannerTableIndexColumn is a single column entry in an index.
type SpannerTableIndexColumn struct {
	// The name of the column
	Name string
	// The sort order of the column in the index
	//
	// Accepts either SpannerTableIndexColumnOrderAsc or SpannerTableIndexColumnOrderDesc
	Order SpannerTableIndexColumnOrder
}

// SpannerTableIndex represents a Spanner table index.
type SpannerTableIndex struct {
	// The name of the index
	Name string
	// The columns that make up the index
	Columns []*SpannerTableIndexColumn
	// Whether the index is unique
	Unique *wrapperspb.BoolValue
}

// CreateDdl renders the CREATE INDEX statement for the index on the given
// table. An UNSPECIFIED column order renders as ASC; the input is not mutated.
func (i *SpannerTableIndex) CreateDdl(table string) (string, error) {
	if i == nil {
		return "", nil
	}
	if table == "" {
		return "", fmt.Errorf("table is required for index %s", i.Name)
	}
	if i.Name == "" {
		return "", errors.New("index name is required")
	}
	if len(i.Columns) == 0 {
		return "", fmt.Errorf("at least one column is required for index %s", i.Name)
	}

	unique := ""
	if i.Unique != nil && i.Unique.GetValue() {
		unique = "UNIQUE"
	}

	columns := make([]string, 0, len(i.Columns))
	for _, column := range i.Columns {
		order := column.Order
		if order == SpannerTableIndexColumnOrderUnspecified {
			order = SpannerTableIndexColumnOrderAsc
		}
		columns = append(columns, fmt.Sprintf("%s %s", column.Name, strings.ToUpper(order.String())))
	}

	return fmt.Sprintf("CREATE %s INDEX %s ON %s (%s)",
		unique,
		i.Name,
		table,
		strings.Join(columns, ", "),
	), nil
}

// DropIndexDdl renders the DROP INDEX statement.
func DropIndexDdl(name string) string {
	return "DROP INDEX " + name
}
