package schema

import "fmt"

// SpannerTableSchema represents the schema of a Spanner table.
type SpannerTableSchema struct {
	// The columns that make up the table schema.
	Columns []*SpannerTableColumn
}

func (s *SpannerTableSchema) GetColumns() []*SpannerTableColumn {
	if s == nil {
		return nil
	}

	return s.Columns
}

func (s *SpannerTableSchema) GetPrimaryKeyColumns() []string {
	if s == nil {
		return nil
	}

	var primaryKeys []string
	for _, column := range s.GetColumns() {
		if column.PrimaryKey() {
			primaryKeys = append(primaryKeys, fmt.Sprintf("`%s`", column.GetName()))
		}
	}

	return primaryKeys
}
