package services

import (
	"context"
	"sort"

	"terraform-provider-alis/internal/spanner/conn"

	_ "github.com/googleapis/go-sql-spanner"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func GetIndexes(ctx context.Context, cn conn.Connection, database string, tableName string) ([]*SpannerTableIndex, error) {
	// Get the indexes for the table. "" is Spanner's default schema.
	var results []*Index
	if err := cn.Query(ctx, database, &results,
		"SELECT i.index_name,"+
			"i.is_unique,"+
			"i.index_type,"+
			"ic.ordinal_position,"+
			"ic.column_ordering,"+
			"ic.is_nullable,"+
			"col.column_name"+
			" FROM information_schema.indexes i"+
			" LEFT JOIN information_schema.index_columns ic ON ic.table_name = i.table_name AND ic.index_name = i.index_name"+
			" LEFT JOIN information_schema.columns col ON col.column_name = ic.column_name AND col.table_name = ic.table_name"+
			" WHERE i.index_name IS NOT NULL AND i.table_schema = ? AND i.table_name = ?",
		"", tableName,
	); err != nil {
		return nil, err
	}

	resultsMap := map[string]map[string]*Index{}
	for _, r := range results {
		if _, ok := resultsMap[r.IndexName]; !ok {
			resultsMap[r.IndexName] = map[string]*Index{}
		}
		resultsMap[r.IndexName][r.ColumnName] = r
	}

	indexMap := make(map[string]*SpannerTableIndex)
	for _, r := range results {
		if r.IndexType == "PRIMARY_KEY" {
			continue
		}

		idx, ok := indexMap[r.IndexName]
		if !ok {
			idx = &SpannerTableIndex{
				Name:    r.IndexName,
				Columns: []*SpannerTableIndexColumn{},
				Unique:  wrapperspb.Bool(r.IsUnique),
			}
		}
		var order SpannerTableIndexColumnOrder
		switch r.ColumnOrdering {
		case "ASC":
			order = SpannerTableIndexColumnOrder_ASC
		case "DESC":
			order = SpannerTableIndexColumnOrder_DESC
		}
		idx.Columns = append(idx.Columns, &SpannerTableIndexColumn{
			Name:  r.ColumnName,
			Order: order,
		})
		indexMap[r.IndexName] = idx
	}

	indexes := make([]*SpannerTableIndex, 0)
	for _, idx := range indexMap {
		// Sort the columns by ordinal position
		sort.Slice(idx.Columns, func(i, j int) bool {
			return resultsMap[idx.Name][idx.Columns[i].Name].OrdinalPosition < resultsMap[idx.Name][idx.Columns[j].Name].OrdinalPosition
		})

		// Append the index to the list
		indexes = append(indexes, idx)
	}

	return indexes, nil
}
