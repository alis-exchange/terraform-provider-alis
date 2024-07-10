package services

import (
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

type SpannerTableForeignKey struct {
	// Referenced table
	ReferencedTable string
	// Referenced column
	ReferencedColumn string
	// Referencing column
	Column string
}

type SpannerTableForeignKeysConstraint struct {
	// The name of the constraint
	Name string
	// Foreign keys
	ForeignKeys []*SpannerTableForeignKey
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

// SpannerTableColumn represents a Spanner table column.
type SpannerTableColumn struct {
	// The name of the column.
	//
	// Must be unique within the table.
	Name string
	// Whether the column is a primary key
	IsPrimaryKey *wrapperspb.BoolValue
	// Whether the column is auto-incrementing
	// This is typically paired with is_primary_key=true
	// This is only valid for numeric columns i.e. INT64, FLOAT64
	AutoIncrement *wrapperspb.BoolValue
	// Whether the values in the column are unique
	Unique *wrapperspb.BoolValue
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
