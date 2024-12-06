package services

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type SpannerTableIndexColumn struct {
	// The name of the column
	Name string
	// The sort order of the column in the index
	//
	// Accepts either SpannerTableIndexColumnOrder_ASC or SpannerTableIndexColumnOrder_DESC
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
	OnDelete SpannerTableForeignKeyConstraintAction
}

// ProtoFileDescriptorSet represents a Proto File Descriptor Set.
type ProtoFileDescriptorSet struct {
	// Proto package.
	// Typically paired with PROTO columns.
	ProtoPackage *wrapperspb.StringValue
	// Proto File Descriptor Set file path.
	// Typically paired with PROTO columns.
	FileDescriptorSetPath *wrapperspb.StringValue
	// Proto File Descriptor Set file source.
	// Typically paired with PROTO columns.
	FileDescriptorSetPathSource ProtoFileDescriptorSetSource
	// Proto File Descriptor Set bytes.
	fileDescriptorSet *descriptorpb.FileDescriptorSet
}

type SpannerTableAutoGeneratedColumn struct {
	// The columns that make up the auto-generated column
	Columns []string
}

// SpannerTableColumn represents a Spanner table column.
type SpannerTableColumn struct {
	// The name of the column.
	//
	// Must be unique within the table.
	Name string
	// Whether the column is a primary key
	IsPrimaryKey *wrapperspb.BoolValue
	// Whether the column is a generated/computed/stored value from
	// other columns in the table
	IsComputed *wrapperspb.BoolValue
	// The expression for the computed column
	// This is only valid for computed columns
	ComputationDdl *wrapperspb.StringValue
	// Whether the column is auto-incrementing
	// This is typically paired with is_primary_key=true
	// This is only valid for numeric columns i.e. INT64, FLOAT64
	AutoIncrement *wrapperspb.BoolValue
	// Whether the values in the column are unique
	Unique *wrapperspb.BoolValue
	// Whether the column should auto-generate a create time
	// This is only valid for TIMESTAMP columns
	AutoCreateTime *wrapperspb.BoolValue
	// Whether the column should auto-generate an update time
	// This is only valid for TIMESTAMP columns
	AutoUpdateTime *wrapperspb.BoolValue
	// The type of the column
	Type string
	// The maximum size of the column.
	//
	// For STRING columns, this is the maximum length of the column in characters.
	// For BYTES columns, this is the maximum length of the column in bytes.
	Size *wrapperspb.Int64Value
	// The precision of the column.
	// This is typically paired with numeric columns i.e. INT64, FLOAT64
	Precision *wrapperspb.Int64Value
	// The scale of the column.
	// This is typically paired with numeric columns i.e. INT64, FLOAT64
	Scale *wrapperspb.Int64Value
	// Whether the column is nullable
	Required *wrapperspb.BoolValue
	// The default value of the column.
	//
	// Accepts any type of value given that the value is valid for the column type.
	DefaultValue *wrapperspb.StringValue
	// The proto file descriptor set for the column.
	//
	// This is typically paired with PROTO columns.
	ProtoFileDescriptorSet *ProtoFileDescriptorSet
}

// SpannerTableSchema represents the schema of a Spanner table.
type SpannerTableSchema struct {
	// The columns that make up the table schema.
	Columns []*SpannerTableColumn
}

// SpannerTable represents a Spanner table.
type SpannerTable struct {
	// Fully qualified name of the table.
	// Format: projects/{project}/instances/{instance}/databases/{database}/tables/{table}
	Name string
	// The schema of the table.
	Schema *SpannerTableSchema
}

// TablePolicyBindingPermission represents a Spanner table role binding permission.
type TablePolicyBindingPermission int64

const (
	TablePolicyBindingPermission_UNSPECIFIED TablePolicyBindingPermission = iota
	TablePolicyBindingPermission_SELECT
	TablePolicyBindingPermission_INSERT
	TablePolicyBindingPermission_UPDATE
	TablePolicyBindingPermission_DELETE
)

func (t TablePolicyBindingPermission) String() string {
	return [...]string{"UNSPECIFIED", "SELECT", "INSERT", "UPDATE", "DELETE"}[t]
}

// SpannerTablePolicyBindingPermissions is a list of all Spanner table role binding permissions.
var SpannerTablePolicyBindingPermissions = []string{
	TablePolicyBindingPermission_SELECT.String(),
	TablePolicyBindingPermission_INSERT.String(),
	TablePolicyBindingPermission_UPDATE.String(),
	TablePolicyBindingPermission_DELETE.String(),
}

// TablePolicyBinding represents a Spanner table role binding.
type TablePolicyBinding struct {
	// The role to which permissions are assigned.
	Role string
	// The permissions to grant to role.
	//
	Permissions []TablePolicyBindingPermission
}

// TablePolicy represents a Spanner table roles policy.
type TablePolicy struct {
	Bindings []*TablePolicyBinding
}

type TablePermissionsRow struct {
	TABLE_NAME     string
	PRIVILEGE_TYPE string
	GRANTEE        string
}

func (r TablePermissionsRow) GetPermission() TablePolicyBindingPermission {
	switch r.PRIVILEGE_TYPE {
	case "SELECT":
		return TablePolicyBindingPermission_SELECT
	case "INSERT":
		return TablePolicyBindingPermission_INSERT
	case "UPDATE":
		return TablePolicyBindingPermission_UPDATE
	case "DELETE":
		return TablePolicyBindingPermission_DELETE
	default:
		return TablePolicyBindingPermission_UNSPECIFIED
	}
}

type ColumnMetadataMeta struct {
	Type                        string `json:"type"`
	Size                        string `json:"size"`
	Precision                   string `json:"precision"`
	Scale                       string `json:"scale"`
	Required                    string `json:"required"`
	AutoIncrement               string `json:"auto_increment"`
	Unique                      string `json:"unique"`
	AutoCreateTime              string `json:"auto_create_time"`
	AutoUpdateTime              string `json:"auto_update_time"`
	DefaultValue                string `json:"default_value"`
	IsPrimaryKey                string `json:"is_primary_key"`
	IsComputed                  string `json:"is_computed"`
	ComputationDdl              string `json:"computation_ddl"`
	ProtoPackage                string `json:"proto_package"`
	FileDescriptorSetPath       string `json:"file_descriptor_set_path"`
	FileDescriptorSetPathSource string `json:"file_descriptor_set_path_source"`
}

// Scan scans value into Jsonb and implements sql.Scanner interface
func (c *ColumnMetadataMeta) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &c)
}

// Value returns value of CustomerInfo struct and implements driver.Valuer interface
func (c ColumnMetadataMeta) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c ColumnMetadataMeta) GormDataType() string {
	return "bytes"
}

type ColumnMetadata struct {
	TableName  string              `gorm:"primaryKey"`
	ColumnName string              `gorm:"primaryKey"`
	Metadata   *ColumnMetadataMeta `gorm:"type:bytes"`
	CreatedAt  time.Time           // Automatically managed by GORM for creation time
	UpdatedAt  time.Time           // Automatically managed by GORM for update time
}

type Index struct {
	IndexName       string
	IndexType       string
	ColumnName      string
	ColumnOrdering  string
	IsUnique        bool
	OrdinalPosition int
}

type Constraint struct {
	CONSTRAINT_NAME    string
	CONSTRAINT_TYPE    string
	CONSTRAINED_TABLE  string
	CONSTRAINED_COLUMN string
	UPDATE_RULE        string
	DELETE_RULE        string
	REFERENCED_TABLE   string
	REFERENCED_COLUMN  string
}

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
	SpannerTableDataType_PROTO
	SpannerTableDataType_STRING_ARRAY
	SpannerTableDataType_INT64_ARRAY
	SpannerTableDataType_FLOAT32_ARRAY
	SpannerTableDataType_FLOAT64_ARRAY
)

func (t SpannerTableDataType) String() string {
	return [...]string{"BOOL", "INT64", "FLOAT64", "STRING", "BYTES", "DATE", "TIMESTAMP", "JSON", "PROTO",
		"ARRAY<STRING>", "ARRAY<INT64>", "ARRAY<FLOAT32>", "ARRAY<FLOAT64>"}[t-1]
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
	SpannerTableDataType_PROTO.String(),
	SpannerTableDataType_STRING_ARRAY.String(),
	SpannerTableDataType_INT64_ARRAY.String(),
	SpannerTableDataType_FLOAT32_ARRAY.String(),
	SpannerTableDataType_FLOAT64_ARRAY.String(),
}

type SpannerTableIndexColumnOrder int64

const (
	SpannerTableIndexColumnOrder_UNSPECIFIED SpannerTableIndexColumnOrder = iota
	SpannerTableIndexColumnOrder_ASC
	SpannerTableIndexColumnOrder_DESC
)

type ProtoFileDescriptorSetSource int64

const (
	ProtoFileDescriptorSetSourceUNSPECIFIED ProtoFileDescriptorSetSource = iota
	ProtoFileDescriptorSetSourceGcs
	ProtoFileDescriptorSetSourceUrl
)

func (s SpannerTableIndexColumnOrder) String() string {
	return [...]string{"unspecified", "asc", "desc"}[s]
}

var SpannerTableIndexColumnOrders = []string{
	SpannerTableIndexColumnOrder_ASC.String(),
	SpannerTableIndexColumnOrder_DESC.String(),
}

type SpannerTableForeignKeyConstraintAction int64

const (
	SpannerTableForeignKeyConstraintActionUnspecified SpannerTableForeignKeyConstraintAction = iota
	SpannerTableForeignKeyConstraintActionCascade
	SpannerTableForeignKeyConstraintNoAction
)

func (a SpannerTableForeignKeyConstraintAction) String() string {
	return [...]string{"", "CASCADE", "NO ACTION"}[a]
}

func SpannerTableForeignKeyConstraintActionFromString(s string) SpannerTableForeignKeyConstraintAction {
	switch s {
	case "CASCADE":
		return SpannerTableForeignKeyConstraintActionCascade
	case "NO ACTION":
		return SpannerTableForeignKeyConstraintNoAction
	default:
		return SpannerTableForeignKeyConstraintActionUnspecified
	}
}

var SpannerTableForeignKeyConstraintActions = []string{
	SpannerTableForeignKeyConstraintActionCascade.String(),
	SpannerTableForeignKeyConstraintNoAction.String(),
}

type SpannerTableRowDeletionPolicy struct {
	// The name of the TIMESTAMP column that is used to determine when a row is deleted
	Column string
	// The duration after which a row is deleted in days
	Duration *wrapperspb.Int64Value
}
