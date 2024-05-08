package services

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dynamicstruct "github.com/ompluscator/dynamic-struct"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"terraform-provider-alis/internal/utils"
)

// SpannerTableDataType is a type for Spanner table column data types.
type SpannerTableDataType int64

const (
	SpannerTableDataType_BOOL SpannerTableDataType = iota + 1
	SpannerTableDataType_INT64
	SpannerTableDataType_FLOAT64
	SpannerTableDataType_STRING
	SpannerTableDataType_BYTES
	SpannerTableDataType_DATE
	SpannerTableDataType_TIMESTAMP
	SpannerTableDataType_JSON
)

func (t SpannerTableDataType) String() string {
	return [...]string{"BOOL", "INT64", "FLOAT64", "STRING", "BYTES", "DATE", "TIMESTAMP", "JSON"}[t-1]
}

// SpannerTableDataTypes is a list of all Spanner table column data types.
var SpannerTableDataTypes = []string{
	SpannerTableDataType_BOOL.String(),
	SpannerTableDataType_INT64.String(),
	SpannerTableDataType_FLOAT64.String(),
	SpannerTableDataType_STRING.String(),
	SpannerTableDataType_BYTES.String(),
	SpannerTableDataType_DATE.String(),
	SpannerTableDataType_TIMESTAMP.String(),
	SpannerTableDataType_JSON.String(),
}

type SpannerTableIndexColumnOrder int64

const (
	SpannerTableIndexColumnOrder_UNSPECIFIED SpannerTableIndexColumnOrder = iota
	SpannerTableIndexColumnOrder_ASC
	SpannerTableIndexColumnOrder_DESC
)

func (s SpannerTableIndexColumnOrder) String() string {
	return [...]string{"unspecified", "asc", "desc"}[s]
}

var SpannerTableIndexColumnOrders = []string{
	SpannerTableIndexColumnOrder_ASC.String(),
	SpannerTableIndexColumnOrder_DESC.String(),
}

type Index struct {
	IndexName       string
	IndexType       string
	ColumnName      string
	ColumnOrdering  string
	IsUnique        bool
	OrdinalPosition int
}

func GetIndexes(db *gorm.DB, tableName string) ([]*SpannerTableIndex, error) {

	currentDatabase := db.Migrator().CurrentDatabase()
	// Get the indexes for the table
	var results []*Index
	db = db.Raw(
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
		currentDatabase, tableName,
	).Scan(&results)

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

func CreateIndex(db *gorm.DB, tableName string, index *SpannerTableIndex) error {
	unique := ""
	if index.Unique != nil && index.Unique.GetValue() {
		unique = "UNIQUE"
	}
	columns := make([]string, 0)
	for _, column := range index.Columns {

		if column.Order == SpannerTableIndexColumnOrder_UNSPECIFIED {
			column.Order = SpannerTableIndexColumnOrder_ASC
		}

		columns = append(columns, fmt.Sprintf("%s %s", column.Name, column.Order.String()))
	}

	// Create the index
	if err := db.Exec(fmt.Sprintf("CREATE %s INDEX %s ON %s (%s)",
		unique,
		index.Name,
		tableName,
		strings.Join(columns, ", "),
	)).Error; err != nil {
		return err
	}

	return nil
}

// ParseSchemaToStruct parses a *v1.SpannerTable_Schema to a struct that can be used with gorm
func ParseSchemaToStruct(schema *SpannerTableSchema) (interface{}, error) {
	// Ensure the schema is not nil
	if schema == nil {
		return nil, errors.New("schema is nil")
	}
	// Ensure the schema has columns
	if len(schema.Columns) == 0 {
		return nil, errors.New("schema requires at least one column")
	}

	// Create a new dynamic struct
	instance := dynamicstruct.NewStruct()

	type ColumnIndex struct {
		Name     string
		Unique   bool
		Priority int
		Order    string
	}

	type ColumnIndices struct {
		Indices []*ColumnIndex
	}

	// Create a map of column indices
	// Keys are column names
	columnIndices := map[string]*ColumnIndices{}
	if schema.Indices != nil && len(schema.Indices) > 0 {
		// Iterate over the indices and add them to the map
		for i, index := range schema.Indices {
			if index.Columns != nil && len(index.Columns) > 0 {
				for _, column := range index.Columns {
					if _, ok := columnIndices[column.Name]; !ok {
						columnIndices[column.Name] = &ColumnIndices{}
					}

					// Check if the index is unique
					var unique bool
					if index.Unique != nil {
						unique = index.Unique.GetValue()
					}
					// Add the index to the column
					columnIndices[column.Name].Indices = append(columnIndices[column.Name].Indices, &ColumnIndex{
						Name:     index.Name,
						Unique:   unique,
						Priority: i + 1,
						Order:    column.Order.String(),
					})
				}
			}
		}
	}

	// Iterate over the columns and add them to the struct
	for _, column := range schema.Columns {
		// `gorm:"column"`
		// `gorm:"primaryKey"`
		// `gorm:"unique"`
		// `gorm:"default"`
		// `gorm:"precision"`
		// `gorm:"scale"`
		// `gorm:"not null"`
		// `gorm:"autoCreateTime"`
		// `gorm:"autoUpdateTime"`
		// `gorm:"index"`
		// `gorm:"size"`
		gormTags := make([]string, 0)
		// Add column name
		gormTags = append(gormTags, fmt.Sprintf("column:%s", column.Name))
		// Check if the column is a primary key
		if column.IsPrimaryKey != nil && column.IsPrimaryKey.GetValue() {
			gormTags = append(gormTags, "primaryKey")
		}
		// Set auto increment
		if column.AutoIncrement != nil && column.AutoIncrement.GetValue() {
			gormTags = append(gormTags, "autoIncrement:true")
		} else {
			gormTags = append(gormTags, "autoIncrement:false")
		}
		// Check if unique is set
		if column.Unique != nil && column.Unique.GetValue() {
			gormTags = append(gormTags, "unique")
		}
		// Check if a default value is set
		if column.DefaultValue != nil && column.DefaultValue.GetValue() != "" {
			gormTags = append(gormTags, fmt.Sprintf("default:%v", column.DefaultValue.GetValue()))
		}
		// Check if size is specified
		if column.Size != nil {
			gormTags = append(gormTags, fmt.Sprintf("size:%v", column.Size.GetValue()))
		}
		// Check if precision is specified
		if column.Precision != nil {
			gormTags = append(gormTags, fmt.Sprintf("precision:%v", column.Precision.GetValue()))
		}
		// Check if scale is specified
		if column.Scale != nil {
			gormTags = append(gormTags, fmt.Sprintf("scale:%v", column.Scale.GetValue()))
		}
		// Check if the column is nullable
		if column.Required != nil && column.Required.GetValue() {
			gormTags = append(gormTags, "not null")
		}
		// Check if the column has any indices
		if indices, ok := columnIndices[column.Name]; ok {
			for _, index := range indices.Indices {
				tag := fmt.Sprintf("index:%s,sort:%s,priority:%d", index.Name, index.Order, index.Priority)
				// Check if the index is unique
				if index.Unique {
					tag += ",unique"
				}
				gormTags = append(gormTags, tag)
			}
		}

		tags := strings.Join(gormTags, ";")

		pascalCaseColumnName := utils.SnakeCaseToPascalCase(column.Name)
		switch column.Type {
		case SpannerTableDataType_BOOL.String():
			instance.AddField(pascalCaseColumnName, false, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_INT64.String():
			instance.AddField(pascalCaseColumnName, int64(0), fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_FLOAT64.String():
			instance.AddField(pascalCaseColumnName, float64(0), fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_STRING.String():
			instance.AddField(pascalCaseColumnName, "", fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_BYTES.String():
			instance.AddField(pascalCaseColumnName, []byte{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_DATE.String():
			instance.AddField(pascalCaseColumnName, datatypes.Date{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_TIMESTAMP.String():
			instance.AddField(pascalCaseColumnName, time.Time{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_JSON.String():
			instance.AddField(pascalCaseColumnName, datatypes.JSON{}, fmt.Sprintf("gorm:\"%s\"", tags))
		default:
			return nil, errors.New("unknown column type")
		}
	}

	return instance.Build().New(), nil
}
