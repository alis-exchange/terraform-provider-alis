package schema

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/spanner/names"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// SpannerTable represents a Spanner table.
type SpannerTable struct {
	// Fully qualified name of the table.
	// Format: projects/{project}/instances/{instance}/databases/{database}/tables/{table}
	Name string
	// The schema of the table.
	Schema *SpannerTableSchema
	// The table interleave.
	Interleave *SpannerTableInterleave
}

// GetProject returns "projects/{p}", or "" when the table name is unset or
// malformed (no panics on short names).
func (t *SpannerTable) GetProject() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return "projects/" + n.Project
}

// GetProjectId returns the project id segment, or "" when the table name is
// unset or malformed.
func (t *SpannerTable) GetProjectId() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return n.Project
}

// GetInstance returns "projects/{p}/instances/{i}", or "" when the table
// name is unset or malformed.
func (t *SpannerTable) GetInstance() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return fmt.Sprintf("projects/%s/instances/%s", n.Project, n.Instance)
}

// GetInstanceId returns the instance id segment, or "" when the table name
// is unset or malformed.
func (t *SpannerTable) GetInstanceId() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return n.Instance
}

// GetDatabase returns the fully qualified database name, or "" when the
// table name is unset or malformed.
func (t *SpannerTable) GetDatabase() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return n.DatabaseName().String()
}

// GetDatabaseId returns the database id segment, or "" when the table name
// is unset or malformed.
func (t *SpannerTable) GetDatabaseId() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return n.Database
}

// GetTableId returns the table id segment, or "" when the table name is
// unset or malformed.
func (t *SpannerTable) GetTableId() string {
	n, err := names.ParseTable(t.GetName())
	if err != nil {
		return ""
	}
	return n.Table
}

func (t *SpannerTable) GetName() string {
	if t == nil {
		return ""
	}

	return t.Name
}

func (t *SpannerTable) GetSchema() *SpannerTableSchema {
	if t == nil {
		return nil
	}

	return t.Schema
}

func (t *SpannerTable) GetInterleave() *SpannerTableInterleave {
	if t == nil {
		return nil
	}

	return t.Interleave
}

// CreateDdl renders the CREATE TABLE statement, including primary key and
// interleave clauses.
func (t *SpannerTable) CreateDdl() (string, error) {
	ddl := fmt.Sprintf("CREATE TABLE `%s` (", t.GetTableId())

	// Add columns
	{
		var columnsDdls []string
		for _, column := range t.GetSchema().GetColumns() {
			columnDdl, err := column.ddl()
			if err != nil {
				return "", err
			}
			columnsDdls = append(columnsDdls, columnDdl)
		}
		ddl += strings.Join(columnsDdls, ", ")
	}

	ddl += ")"

	// Add primary key
	{
		primaryKeys := t.GetSchema().GetPrimaryKeyColumns()
		if len(primaryKeys) > 0 {
			ddl += fmt.Sprintf(" PRIMARY KEY (%s)", strings.Join(primaryKeys, ", "))
		}
	}

	// Add interleave
	if interleaveDdl := t.GetInterleave().ddl(); interleaveDdl != "" {
		ddl += ", " + interleaveDdl
	}

	return ddl, nil
}

// AlterDdl diffs the table against its existing state and renders the ALTER
// statements plus the list of dropped columns. Statement order is
// deterministic: drops, then adds, then in-place alters, each sorted by
// column name — the batch is submitted to UpdateDatabaseDdl as-is, so a
// stable order keeps applies reproducible.
func (t *SpannerTable) AlterDdl(existingTable *SpannerTable) ([]string, []*SpannerTableColumn, error) {
	// If either table is nil, return gracefully.
	if t == nil || existingTable == nil {
		return nil, nil, nil
	}

	var statements []string

	// Map existing columns for easy lookup
	existingColumnsMap := make(map[string]*SpannerTableColumn)
	for _, column := range existingTable.GetSchema().GetColumns() {
		existingColumnsMap[column.GetName()] = column
	}

	// Map updated columns for easy lookup
	updatedColumnsMap := make(map[string]*SpannerTableColumn)
	for _, column := range t.GetSchema().GetColumns() {
		updatedColumnsMap[column.GetName()] = column
	}

	sortedNames := func(m map[string]*SpannerTableColumn) []string {
		names := make([]string, 0, len(m))
		for name := range m {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}
	existingNames := sortedNames(existingColumnsMap)
	updatedNames := sortedNames(updatedColumnsMap)

	// Find columns to drop(only existing columns)
	var dropColumns []*SpannerTableColumn
	for _, name := range existingNames {
		if _, exists := updatedColumnsMap[name]; !exists {
			statements = append(statements, fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", t.GetTableId(), name))
			dropColumns = append(dropColumns, existingColumnsMap[name])
		}
	}

	// Find columns to add(only new columns)
	for _, name := range updatedNames {
		if _, exists := existingColumnsMap[name]; !exists {
			columnDdl, err := updatedColumnsMap[name].ddl()
			if err != nil {
				return nil, nil, err
			}
			statements = append(statements, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", t.GetTableId(), columnDdl))
		}
	}

	// Find columns to modify
	for _, name := range updatedNames {
		updatedColumn := updatedColumnsMap[name]
		existingColumn, exists := existingColumnsMap[name]
		if !exists {
			continue
		}

		// Compare columns
		if !updatedColumn.compare(existingColumn) {
			alterColumnDdls, err := updatedColumn.alterDdl(existingColumn)
			if err != nil {
				return nil, nil, err
			}

			if len(alterColumnDdls) > 0 {
				for _, alterColumnDdl := range alterColumnDdls {
					statements = append(statements, fmt.Sprintf("ALTER TABLE `%s` ALTER COLUMN %s", t.GetTableId(), alterColumnDdl))
				}
			}
		}
	}

	return statements, dropColumns, nil
}

// DeleteDdl renders the DROP TABLE statement.
func (t *SpannerTable) DeleteDdl() (string, error) {
	return fmt.Sprintf("DROP TABLE `%s`", t.GetTableId()), nil
}

// Create creates the table in Spanner.
func (t *SpannerTable) Create(ctx context.Context, cn conn.Connection) (*SpannerTable, error) {
	// If table is nil, return gracefully.
	if t == nil {
		return t, nil
	}

	// Generate table DDL
	ddl, err := t.CreateDdl()
	if err != nil {
		return nil, err
	}

	// Update the database schema.
	if err := cn.ExecuteDDL(ctx, t.GetDatabase(), ddl); err != nil {
		return nil, err
	}

	return t, nil
}

// tableInfoRow, informationSchemaColumnRow, primaryKeyRow, and
// columnOptionRow mirror the INFORMATION_SCHEMA result shapes;
// sql.NullString distinguishes NULL from empty strings.
type tableInfoRow struct {
	TableName       sql.NullString `gorm:"column:TABLE_NAME"`
	ParentTableName sql.NullString `gorm:"column:PARENT_TABLE_NAME"`
	OnDeleteAction  sql.NullString `gorm:"column:ON_DELETE_ACTION"`
	InterleaveType  sql.NullString `gorm:"column:INTERLEAVE_TYPE"`
}

type informationSchemaColumnRow struct {
	ColumnName     sql.NullString `gorm:"column:COLUMN_NAME"`
	SpannerType    sql.NullString `gorm:"column:SPANNER_TYPE"`
	IsNullable     sql.NullString `gorm:"column:IS_NULLABLE"`
	ColumnDefault  sql.NullString `gorm:"column:COLUMN_DEFAULT"`
	IsGenerated    sql.NullString `gorm:"column:IS_GENERATED"`
	IsStored       sql.NullString `gorm:"column:IS_STORED"`
	GenerationExpr sql.NullString `gorm:"column:GENERATION_EXPRESSION"`
}

type primaryKeyRow struct {
	ColumnName sql.NullString `gorm:"column:COLUMN_NAME"`
}

type columnOptionRow struct {
	ColumnName  sql.NullString `gorm:"column:COLUMN_NAME"`
	OptionName  sql.NullString `gorm:"column:OPTION_NAME"`
	OptionValue sql.NullString `gorm:"column:OPTION_VALUE"`
}

// Get hydrates the table from the database's INFORMATION_SCHEMA (TABLES,
// COLUMNS, INDEX_COLUMNS, and COLUMN_OPTIONS), returning ErrTableNotFound
// when the table does not exist. name must be the fully qualified table
// name; it is adopted when the receiver is nil or unnamed. Proto columns
// surface as Type "PROTO" with ProtoPackage carrying the fully-qualified
// message name, and an allow_commit_timestamp column option maps to
// AutoUpdateTime.
func (t *SpannerTable) Get(ctx context.Context, cn conn.Connection, name string) (*SpannerTable, error) {
	// If table is nil, initialize it.
	if t == nil || t.GetName() == "" {
		t = &SpannerTable{
			Name:   name,
			Schema: &SpannerTableSchema{},
		}
	}

	// Check INFORMATION_SCHEMA for table
	var interleave *SpannerTableInterleave
	{
		var row tableInfoRow
		if err := cn.Query(ctx, t.GetDatabase(), &row,
			`SELECT TABLE_NAME,PARENT_TABLE_NAME,ON_DELETE_ACTION,INTERLEAVE_TYPE FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ?`,
			t.GetTableId()); err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, ErrTableNotFound{
					table: t.GetName(),
					err:   err,
				}
			}
			return nil, err
		}

		if row.ParentTableName.Valid && row.ParentTableName.String != "" && row.InterleaveType.Valid && row.InterleaveType.String != "" {
			interleave = &SpannerTableInterleave{
				ParentTable: row.ParentTableName.String,
			}

			if row.InterleaveType.String == "IN PARENT" {
				interleave.OnDelete = SpannerTableConstraintActionFromString(row.OnDeleteAction.String)
			}
		}
	}

	// Get columns from INFORMATION_SCHEMA
	var columns []*SpannerTableColumn
	{
		var rows []*informationSchemaColumnRow
		if err := cn.Query(
			ctx,
			t.GetDatabase(),
			&rows,
			`SELECT COLUMN_NAME,SPANNER_TYPE,IS_NULLABLE,COLUMN_DEFAULT,IS_GENERATED,IS_STORED,GENERATION_EXPRESSION FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? ORDER BY ORDINAL_POSITION`,
			t.GetTableId(),
		); err != nil {
			return nil, err
		}

		for _, r := range rows {
			columnName := r.ColumnName
			spannerType := r.SpannerType
			isNullable := r.IsNullable
			columnDefault := r.ColumnDefault
			isGenerated := r.IsGenerated
			isStored := r.IsStored
			generationExpr := r.GenerationExpr

			column := &SpannerTableColumn{
				Name: columnName.String,
			}

			// Handle Type
			if spannerType.Valid {
				column.Type = parseSpannerType(spannerType.String)
			}

			// Handle Size
			size := parseSpannerSize(spannerType.String)
			if size != "" && size != "MAX" {
				sizeInt64, err := strconv.ParseInt(size, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid column size: %w", err)
				}

				column.Size = wrapperspb.Int64(sizeInt64)
			}

			// Handle Proto Package
			if protoPackage := parseSpannerProtoPackage(spannerType.String); protoPackage != "" {
				column.ProtoPackage = wrapperspb.String(protoPackage)
			}

			// Handle Nullable
			if isNullable.Valid {
				column.Required = wrapperspb.Bool(isNullable.String == "NO")
			}

			// Handle Default
			if columnDefault.Valid {
				column.DefaultValue = wrapperspb.String(columnDefault.String)
			}

			// Handle Generated
			if isGenerated.Valid {
				column.IsComputed = wrapperspb.Bool(isGenerated.String == "ALWAYS")
			}
			if column.GetIsComputed().GetValue() && generationExpr.Valid {
				column.ComputationDdl = wrapperspb.String(generationExpr.String)
			}

			// Handle Stored
			if isStored.Valid {
				column.IsStored = wrapperspb.Bool(isStored.String == "YES")
			}

			columns = append(columns, column)
		}
	}

	// Get primary keys from INFORMATION_SCHEMA
	{
		var rows []*primaryKeyRow
		if err := cn.Query(
			ctx,
			t.GetDatabase(),
			&rows,
			`SELECT COLUMN_NAME, ORDINAL_POSITION FROM INFORMATION_SCHEMA.INDEX_COLUMNS WHERE TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY_KEY' ORDER BY ORDINAL_POSITION`,
			t.GetTableId(),
		); err != nil {
			return nil, err
		}

		var primaryKeys []string
		for _, r := range rows {
			primaryKeys = append(primaryKeys, r.ColumnName.String)
		}

		for _, column := range columns {
			for _, primaryKey := range primaryKeys {
				if column.GetName() == primaryKey {
					column.IsPrimaryKey = wrapperspb.Bool(true)
				}
			}
		}
	}

	// Column options carry what the COLUMNS view does not:
	// allow_commit_timestamp is the DDL form of auto_update_time.
	{
		var rows []*columnOptionRow
		if err := cn.Query(ctx, t.GetDatabase(), &rows,
			`SELECT COLUMN_NAME, OPTION_NAME, OPTION_VALUE FROM INFORMATION_SCHEMA.COLUMN_OPTIONS WHERE TABLE_NAME = ?`,
			t.GetTableId()); err != nil {
			return nil, err
		}

		allowCommitByColumn := map[string]string{}
		for _, r := range rows {
			if r.OptionName.String == "allow_commit_timestamp" {
				allowCommitByColumn[r.ColumnName.String] = r.OptionValue.String
			}
		}
		for _, column := range columns {
			switch allowCommitByColumn[column.GetName()] {
			case "TRUE":
				column.AutoUpdateTime = wrapperspb.Bool(true)
			case "FALSE":
				column.AutoUpdateTime = wrapperspb.Bool(false)
			}
		}
	}

	// Set columns
	if t.GetSchema() == nil {
		t.Schema = &SpannerTableSchema{}
	}
	t.GetSchema().Columns = columns
	t.Interleave = interleave

	return t, nil
}

// Update diffs the table against existingTable and applies the resulting
// ALTER statements (see AlterDdl). It is a no-op when the tables are
// identical or the diff yields no statements.
func (t *SpannerTable) Update(ctx context.Context, cn conn.Connection, existingTable *SpannerTable) (*SpannerTable, error) {
	// If table is nil, return gracefully.
	if t == nil {
		return t, nil
	}

	// If tables are identical, return gracefully
	if t.compare(existingTable) {
		return t, nil
	}

	// Generate alter DDL
	statements, _, err := t.AlterDdl(existingTable)
	if err != nil {
		return nil, err
	}

	// If there are no statements, return gracefully
	if len(statements) == 0 {
		return t, nil
	}

	// Update the database schema.
	if err := cn.ExecuteDDL(ctx, t.GetDatabase(), statements...); err != nil {
		return nil, err
	}

	return t, nil
}

// Delete drops the table from the database.
func (t *SpannerTable) Delete(ctx context.Context, cn conn.Connection) error {
	// If table is nil, return gracefully.
	if t == nil {
		return nil
	}

	// Generate table DDL
	ddl, err := t.DeleteDdl()
	if err != nil {
		return err
	}

	// Update the database schema.
	if err := cn.ExecuteDDL(ctx, t.GetDatabase(), ddl); err != nil {
		return err
	}

	return nil
}

// compare reports whether two tables are identical in name, columns, and
// interleave. Columns are compared positionally, so a reorder counts as a
// difference.
func (t *SpannerTable) compare(other *SpannerTable) bool {
	// If tables are nil, return gracefully.
	if t == nil && other == nil {
		return true
	}

	// If one table is nil, return false.
	if t == nil || other == nil {
		return false
	}

	// Compare table names
	if t.GetName() != other.GetName() {
		return false
	}

	// Compare schemas
	if t.GetSchema() != nil && other.GetSchema() == nil {
		return false
	}
	if t.GetSchema() == nil && other.GetSchema() != nil {
		return false
	}
	if t.GetSchema() != nil && other.GetSchema() != nil {
		if len(t.GetSchema().GetColumns()) != len(other.GetSchema().GetColumns()) {
			return false
		}

		for i, column := range t.GetSchema().GetColumns() {
			if !column.compare(other.GetSchema().GetColumns()[i]) {
				return false
			}
		}
	}

	// Compare interleave
	if t.GetInterleave() != nil && other.GetInterleave() == nil {
		return false
	}
	if t.GetInterleave() == nil && other.GetInterleave() != nil {
		return false
	}
	if t.GetInterleave() != nil && other.GetInterleave() != nil {
		if t.GetInterleave().GetParentTable() != other.GetInterleave().GetParentTable() {
			return false
		}

		if t.GetInterleave().GetOnDelete() != other.GetInterleave().GetOnDelete() {
			return false
		}
	}

	return true
}

// parseSpannerType normalizes a SPANNER_TYPE value from INFORMATION_SCHEMA
// to the provider's type keywords. Accepted shapes: bare scalars ("INT64"),
// sized STRING(n)/BYTES(n), ARRAY<...> of STRING/INT64/FLOAT32/FLOAT64,
// PROTO<...> and ENUM<...>, and a bare or backticked fully-qualified message
// name — the shape proto columns actually arrive in — all of which map to
// "PROTO". Anything else passes through unchanged.
func parseSpannerType(columnType string) string {
	// Handle String types
	if strings.HasPrefix(columnType, "STRING") {
		return "STRING"
	}

	// Handle Byte types
	if strings.HasPrefix(columnType, "BYTES") {
		return "BYTES"
	}

	// Handle ARRAY types
	if strings.HasPrefix(columnType, "ARRAY") {
		// Handle ARRAY<STRING> types
		if strings.HasPrefix(columnType, "ARRAY<STRING") {
			return "ARRAY<STRING>"
		}

		// Handle ARRAY<INT64> types
		if strings.HasPrefix(columnType, "ARRAY<INT64") {
			return "ARRAY<INT64>"
		}

		// Handle ARRAY<FLOAT32> types
		if strings.HasPrefix(columnType, "ARRAY<FLOAT32") {
			return "ARRAY<FLOAT32>"
		}

		// Handle ARRAY<FLOAT64> types
		if strings.HasPrefix(columnType, "ARRAY<FLOAT64") {
			return "ARRAY<FLOAT64>"
		}
	}

	// Handle PROTO types
	if strings.HasPrefix(columnType, "PROTO") {
		return "PROTO"
	}

	// Handle ENUM types
	if strings.HasPrefix(columnType, "ENUM") {
		return "PROTO"
	}

	// INFORMATION_SCHEMA surfaces proto columns as a (possibly backticked)
	// fully-qualified message name rather than a PROTO<...> shape.
	if strings.HasPrefix(columnType, "`") || strings.Contains(columnType, ".") {
		return "PROTO"
	}

	return columnType
}

// parseSpannerSize extracts the declared length from STRING(n), BYTES(n),
// or ARRAY<STRING(n)> types; the result may be "MAX", and is "" for types
// that carry no size.
func parseSpannerSize(columnType string) string {
	// Handle String types
	if strings.HasPrefix(columnType, "STRING") {
		// Remove the prefix and suffix
		trimmedType := strings.TrimSuffix(strings.TrimPrefix(columnType, "STRING("), ")")

		// Return the size
		return trimmedType
	}

	// Handle Byte types
	if strings.HasPrefix(columnType, "BYTES") {
		// Remove the prefix and suffix
		trimmedType := strings.TrimSuffix(strings.TrimPrefix(columnType, "BYTES("), ")")

		// Return the size
		return trimmedType
	}

	// Handle ARRAY<STRING> types
	if strings.HasPrefix(columnType, "ARRAY<STRING") {
		// Remove the prefix and suffix
		trimmedType := strings.TrimSuffix(strings.TrimPrefix(columnType, "ARRAY<STRING("), ")>")

		// Return the size
		return trimmedType
	}

	return ""
}

// parseSpannerProtoPackage extracts the fully-qualified message or enum name
// from a proto column's SPANNER_TYPE: PROTO<...>, ENUM<...>, a backticked
// name, or a bare dotted name. It returns "" for non-proto types.
func parseSpannerProtoPackage(columnType string) string {
	// Handle PROTO types
	if strings.HasPrefix(columnType, "PROTO") {
		// Remove the prefix and suffix
		trimmedType := strings.TrimSuffix(strings.TrimPrefix(columnType, "PROTO<"), ">")

		// Return the proto package
		return trimmedType
	}

	// Handle ENUM types
	if strings.HasPrefix(columnType, "ENUM") {
		// Remove the prefix and suffix
		trimmedType := strings.TrimSuffix(strings.TrimPrefix(columnType, "ENUM<"), ">")

		// Return the proto package
		return trimmedType
	}

	// INFORMATION_SCHEMA surfaces proto columns as a (possibly backticked)
	// fully-qualified message name rather than a PROTO<...> shape.
	if strings.HasPrefix(columnType, "`") {
		return strings.Trim(columnType, "`")
	}
	if strings.Contains(columnType, ".") && !strings.ContainsAny(columnType, "<(") {
		return columnType
	}

	return ""
}
