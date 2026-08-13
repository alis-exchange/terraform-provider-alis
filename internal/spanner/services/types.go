package services

import (
	"terraform-provider-alis/internal/spanner/schema"
)

// The index and row-deletion-policy types are owned by the schema package
// alongside their DDL builders; these aliases keep existing callers compiling.
type (
	SpannerTableIndex             = schema.SpannerTableIndex
	SpannerTableIndexColumn       = schema.SpannerTableIndexColumn
	SpannerTableIndexColumnOrder  = schema.SpannerTableIndexColumnOrder
	SpannerTableRowDeletionPolicy = schema.SpannerTableRowDeletionPolicy
)

const (
	SpannerTableIndexColumnOrder_UNSPECIFIED = schema.SpannerTableIndexColumnOrder_UNSPECIFIED
	SpannerTableIndexColumnOrder_ASC         = schema.SpannerTableIndexColumnOrder_ASC
	SpannerTableIndexColumnOrder_DESC        = schema.SpannerTableIndexColumnOrder_DESC
)

var SpannerTableIndexColumnOrders = schema.SpannerTableIndexColumnOrders

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

type SequenceRow struct {
	Catalog      string `gorm:"column:CATALOG"`
	Schema       string `gorm:"column:SCHEMA"`
	SequenceName string `gorm:"column:SEQUENCE_NAME"`
	DataType     string `gorm:"column:DATA_TYPE"`

	// Pointers handle the potential NULLs from the LEFT JOIN
	OptionName  *string `gorm:"column:OPTION_NAME"`
	OptionValue *string `gorm:"column:OPTION_VALUE"`
	OptionType  *string `gorm:"column:OPTION_TYPE"`
}
