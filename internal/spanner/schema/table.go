package schema

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-alis/internal/spanner/conn"
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

func (t *SpannerTable) GetProject() string {
	if t == nil {
		return ""
	}

	if t.GetName() == "" {
		return ""
	}

	tableNameParts := strings.Split(t.GetName(), "/")
	projectId := tableNameParts[1]

	return fmt.Sprintf("projects/%s", projectId)
}

func (t *SpannerTable) GetProjectId() string {
	if t == nil {
		return ""
	}

	return strings.TrimPrefix(t.GetProject(), "projects/")
}

func (t *SpannerTable) GetInstance() string {
	if t == nil {
		return ""
	}

	if t.GetName() == "" {
		return ""
	}

	tableNameParts := strings.Split(t.GetName(), "/")
	instanceId := tableNameParts[3]

	return fmt.Sprintf("%s/instances/%s", t.GetProject(), instanceId)
}

func (t *SpannerTable) GetInstanceId() string {
	if t == nil {
		return ""
	}

	instanceParts := strings.Split(t.GetInstance(), "/")
	return instanceParts[len(instanceParts)-1]
}

func (t *SpannerTable) GetDatabase() string {
	if t == nil {
		return ""
	}

	if t.GetName() == "" {
		return ""
	}

	tableNameParts := strings.Split(t.GetName(), "/")
	databaseId := tableNameParts[5]

	return fmt.Sprintf("%s/databases/%s", t.GetInstance(), databaseId)
}

func (t *SpannerTable) GetDatabaseId() string {
	if t == nil {
		return ""
	}

	databaseParts := strings.Split(t.GetDatabase(), "/")
	return databaseParts[len(databaseParts)-1]
}

func (t *SpannerTable) GetTableId() string {
	if t == nil {
		return ""
	}

	if t.GetName() == "" {
		return ""
	}

	return strings.Split(t.GetName(), "/")[7]
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

func (t *SpannerTable) createDdl() (string, error) {

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
	{
		interleaveDdl, err := t.GetInterleave().ddl()
		if err != nil {
			return "", err
		}

		if interleaveDdl != "" {
			ddl += fmt.Sprintf(", %s", interleaveDdl)
		}
	}

	return ddl, nil
}

func (t *SpannerTable) alterDdl(existingTable *SpannerTable) ([]string, []*SpannerTableColumn, error) {
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

	// Find columns to drop(only existing columns)
	var dropColumns []*SpannerTableColumn
	for name := range existingColumnsMap {
		if _, exists := updatedColumnsMap[name]; !exists {
			statements = append(statements, fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", t.GetTableId(), name))
			dropColumns = append(dropColumns, existingColumnsMap[name])
		}
	}

	// Find columns to add(only new columns)
	for name, column := range updatedColumnsMap {
		if _, exists := existingColumnsMap[name]; !exists {
			columnDdl, err := column.ddl()
			if err != nil {
				return nil, nil, err
			}
			statements = append(statements, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", t.GetTableId(), columnDdl))
		}
	}

	// Find columns to modify
	for name, updatedColumn := range updatedColumnsMap {
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

func (t *SpannerTable) deleteDdl() (string, error) {
	return fmt.Sprintf("DROP TABLE `%s`", t.GetTableId()), nil
}

// Create creates the table in Spanner.
func (t *SpannerTable) Create(ctx context.Context, cn conn.Connection) (*SpannerTable, error) {
	// If table is nil, return gracefully.
	if t == nil {
		return t, nil
	}

	// Generate table DDL
	ddl, err := t.createDdl()
	if err != nil {
		return nil, err
	}

	// Update the database schema.
	if err := cn.ExecuteDDL(ctx, t.GetDatabase(), ddl); err != nil {
		return nil, err
	}

	if err := UpdateColumnMetadata(ctx, cn, t.GetDatabase(), t.GetTableId(), t.GetSchema().GetColumns()); err != nil {
		// This is not a fatal error, so we log it and continue
		alog.Errorf(ctx, "error updating column metadata: %v", err)
	}

	return t, nil
}

// tableInfoRow, informationSchemaColumnRow, and primaryKeyRow mirror the
// INFORMATION_SCHEMA shapes previously read via a raw spanner.Client with
// spanner.NullString scanning; sql.NullString preserves the NULL semantics.
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

func (t *SpannerTable) Get(ctx context.Context, cn conn.Connection, name string) (*SpannerTable, error) {
	// If table is nil, initialize it.
	if t == nil || t.GetName() == "" {
		t = &SpannerTable{
			Name:   name,
			Schema: &SpannerTableSchema{},
		}
	}

	// Get column metadata
	columnMetadata, err := GetColumnMetadata(ctx, cn, t.GetDatabase(), t.GetTableId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting column metadata: %v", err)
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

	// Get column metadata
	columnMetadataMap := map[string]*ColumnMetadata{}
	for _, metadata := range columnMetadata {
		columnMetadataMap[metadata.ColumnName] = metadata
	}

	// Get columns from INFORMATION_SCHEMA
	var columns []*SpannerTableColumn
	{
		var rows []*informationSchemaColumnRow
		if err := cn.Query(ctx, t.GetDatabase(), &rows,
			`SELECT COLUMN_NAME,SPANNER_TYPE,IS_NULLABLE,COLUMN_DEFAULT,IS_GENERATED,IS_STORED,GENERATION_EXPRESSION FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? ORDER BY ORDINAL_POSITION`,
			t.GetTableId()); err != nil {
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

			if metadata, ok := columnMetadataMap[column.Name]; ok && metadata.Metadata != nil {

				// Populate primary keys if present
				switch metadata.Metadata.IsPrimaryKey {
				case "true":
					column.IsPrimaryKey = wrapperspb.Bool(true)
				case "false":
					column.IsPrimaryKey = wrapperspb.Bool(false)
				}

				// Populate type if present
				switch metadata.Metadata.Type {
				case "nil":
				default:
					column.Type = metadata.Metadata.Type
				}

				// Populate computed if present
				switch metadata.Metadata.IsComputed {
				case "true":
					column.IsComputed = wrapperspb.Bool(true)
				case "false":
					column.IsComputed = wrapperspb.Bool(false)
				}

				// Populate computation ddl if present
				switch metadata.Metadata.ComputationDdl {
				case "nil":
				default:
					column.ComputationDdl = wrapperspb.String(metadata.Metadata.ComputationDdl)
				}

				// Populate stored if present
				switch metadata.Metadata.IsStored {
				case "true":
					column.IsStored = wrapperspb.Bool(true)
				case "false":
					column.IsStored = wrapperspb.Bool(false)
				}

				// Populate auto create time if present
				switch metadata.Metadata.AutoCreateTime {
				case "true":
					column.AutoCreateTime = wrapperspb.Bool(true)
				case "false":
					column.AutoCreateTime = wrapperspb.Bool(false)
				}

				// Populate auto update time if present
				switch metadata.Metadata.AutoUpdateTime {
				case "true":
					column.AutoUpdateTime = wrapperspb.Bool(true)
				case "false":
					column.AutoUpdateTime = wrapperspb.Bool(false)
				}

				// Populate size if present
				switch metadata.Metadata.Size {
				case "nil":
				default:
					// Convert string to int64
					size, err := strconv.ParseInt(metadata.Metadata.Size, 10, 64)
					if err != nil {
						return nil, status.Errorf(codes.Internal, "Error converting size to int64: %v", err)
					}

					column.Size = wrapperspb.Int64(size)
				}

				// Populate nullable if present
				switch metadata.Metadata.Required {
				case "true":
					column.Required = wrapperspb.Bool(true)
				case "false":
					column.Required = wrapperspb.Bool(false)
				}

				// Populate default value if present
				switch metadata.Metadata.DefaultValue {
				case "nil":
				default:
					column.DefaultValue = wrapperspb.String(metadata.Metadata.DefaultValue)
				}

				// Populate proto package
				var protoPackage string
				switch metadata.Metadata.ProtoPackage {
				case "nil":
				default:
					protoPackage = metadata.Metadata.ProtoPackage
				}

				// Populate proto file descriptor set
				var fileDescriptorSetPath string
				switch metadata.Metadata.FileDescriptorSetPath {
				case "nil":
				default:
					fileDescriptorSetPath = metadata.Metadata.FileDescriptorSetPath
				}

				// Populate ProtoFileDescriptorSet
				if protoPackage != "" || fileDescriptorSetPath != "" {
					column.ProtoFileDescriptorSet = &ProtoFileDescriptorSet{}

					if protoPackage != "" {
						column.ProtoFileDescriptorSet.ProtoPackage = wrapperspb.String(protoPackage)
					}

					if fileDescriptorSetPath != "" {
						column.ProtoFileDescriptorSet.FileDescriptorSetPath = wrapperspb.String(fileDescriptorSetPath)
					}
				}
			} else {
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
				protoPackage := parseSpannerProtoPackage(spannerType.String)
				if protoPackage != "" {
					column.ProtoFileDescriptorSet = &ProtoFileDescriptorSet{
						ProtoPackage: wrapperspb.String(protoPackage),
					}
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
			}

			columns = append(columns, column)
		}
	}

	// Get primary keys from INFORMATION_SCHEMA
	{
		var rows []*primaryKeyRow
		if err := cn.Query(ctx, t.GetDatabase(), &rows,
			`SELECT COLUMN_NAME, ORDINAL_POSITION FROM INFORMATION_SCHEMA.INDEX_COLUMNS WHERE TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY_KEY' ORDER BY ORDINAL_POSITION`,
			t.GetTableId()); err != nil {
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

	// Set columns
	if t.GetSchema() == nil {
		t.Schema = &SpannerTableSchema{}
	}
	t.GetSchema().Columns = columns
	t.Interleave = interleave

	return t, nil
}

func (t *SpannerTable) Update(ctx context.Context, cn conn.Connection, existingTable *SpannerTable) (*SpannerTable, error) {
	// If table is nil, return gracefully.
	if t == nil {
		return t, nil
	}

	defer func() {
		// Update column metadata
		if err := UpdateColumnMetadata(ctx, cn, t.GetDatabase(), t.GetTableId(), t.GetSchema().GetColumns()); err != nil {
			// This is not a fatal error, so we log it and continue
			alog.Errorf(ctx, "error updating column metadata: %v", err)
		}
	}()

	// Compare tables
	identical, err := t.compare(existingTable)
	if err != nil {
		return nil, err
	}

	// If tables are identical, return gracefully
	if identical {
		return t, nil
	}

	// Generate alter DDL
	statements, droppedColumns, err := t.alterDdl(existingTable)
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

	// Delete removed columns from column metadata
	if len(droppedColumns) > 0 {
		if err := DeleteColumnMetadata(ctx, cn, t.GetDatabase(), t.GetTableId(), droppedColumns); err != nil {
			// This is not a fatal error, so we log it and continue
			alog.Errorf(ctx, "error deleting column metadata: %v", err)
		}
	}

	return t, nil
}

func (t *SpannerTable) Delete(ctx context.Context, cn conn.Connection) error {
	// If table is nil, return gracefully.
	if t == nil {
		return nil
	}

	// Generate table DDL
	ddl, err := t.deleteDdl()
	if err != nil {
		return err
	}

	// Update the database schema.
	if err := cn.ExecuteDDL(ctx, t.GetDatabase(), ddl); err != nil {
		return err
	}

	// Delete column metadata
	if err := DeleteColumnMetadata(ctx, cn, t.GetDatabase(), t.GetTableId(), []*SpannerTableColumn{}); err != nil {
		// This is not a fatal error, so we log it and continue
		alog.Errorf(ctx, "error deleting column metadata: %v", err)
	}

	return nil
}

func (t *SpannerTable) compare(other *SpannerTable) (bool, error) {
	// If tables are nil, return gracefully.
	if t == nil && other == nil {
		return true, nil
	}

	// If one table is nil, return false.
	if t == nil || other == nil {
		return false, nil
	}

	// Compare table names
	if t.GetName() != other.GetName() {
		return false, nil
	}

	// Compare schemas
	if t.GetSchema() != nil && other.GetSchema() == nil {
		return false, nil
	}
	if t.GetSchema() == nil && other.GetSchema() != nil {
		return false, nil
	}
	if t.GetSchema() != nil && other.GetSchema() != nil {
		if len(t.GetSchema().GetColumns()) != len(other.GetSchema().GetColumns()) {
			return false, nil
		}

		for i, column := range t.GetSchema().GetColumns() {
			if !column.compare(other.GetSchema().GetColumns()[i]) {
				return false, nil
			}
		}
	}

	// Compare interleave
	if t.GetInterleave() != nil && other.GetInterleave() == nil {
		return false, nil
	}
	if t.GetInterleave() == nil && other.GetInterleave() != nil {
		return false, nil
	}
	if t.GetInterleave() != nil && other.GetInterleave() != nil {
		if t.GetInterleave().GetParentTable() != other.GetInterleave().GetParentTable() {
			return false, nil
		}

		if t.GetInterleave().GetOnDelete() != other.GetInterleave().GetOnDelete() {
			return false, nil
		}
	}

	return true, nil
}

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

	return columnType
}

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

	return ""
}
