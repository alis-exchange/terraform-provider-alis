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
	SpannerTableIndexColumnOrderUnspecified = schema.SpannerTableIndexColumnOrderUnspecified
	SpannerTableIndexColumnOrderAsc         = schema.SpannerTableIndexColumnOrderAsc
	SpannerTableIndexColumnOrderDesc        = schema.SpannerTableIndexColumnOrderDesc
)

var SpannerTableIndexColumnOrders = schema.SpannerTableIndexColumnOrders

// TablePolicyBindingPermission represents a Spanner table role binding permission.
type TablePolicyBindingPermission int64

const (
	TablePolicyBindingPermissionUnspecified TablePolicyBindingPermission = iota
	TablePolicyBindingPermissionSelect
	TablePolicyBindingPermissionInsert
	TablePolicyBindingPermissionUpdate
	TablePolicyBindingPermissionDelete
)

func (t TablePolicyBindingPermission) String() string {
	names := [...]string{"UNSPECIFIED", "SELECT", "INSERT", "UPDATE", "DELETE"}
	if t < 0 || int(t) >= len(names) {
		return "UNSPECIFIED"
	}

	return names[t]
}

// TablePolicyBindingPermissions lists every grantable permission. GRANT and
// REVOKE statements iterate it rather than a caller's slice, so their operand
// order is fixed regardless of how the practitioner ordered the config.
var TablePolicyBindingPermissions = []TablePolicyBindingPermission{
	TablePolicyBindingPermissionSelect,
	TablePolicyBindingPermissionInsert,
	TablePolicyBindingPermissionUpdate,
	TablePolicyBindingPermissionDelete,
}

// SpannerTablePolicyBindingPermissions is a list of all Spanner table role binding permissions.
var SpannerTablePolicyBindingPermissions = permissionNames(TablePolicyBindingPermissions)

// permissionNames renders permissions as the identifiers DDL uses.
func permissionNames(permissions []TablePolicyBindingPermission) []string {
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.String())
	}

	return names
}

// TablePolicyBinding represents a Spanner table role binding.
type TablePolicyBinding struct {
	// The role to which permissions are assigned.
	Role string
	// The permissions to grant to role.
	Permissions []TablePolicyBindingPermission
}

// TablePermissionsRow is one row of INFORMATION_SCHEMA.TABLE_PRIVILEGES. The
// column tags are what the query scanner maps on, so they must keep matching
// the column names the query returns.
type TablePermissionsRow struct {
	TableName     string `db:"TABLE_NAME"`
	PrivilegeType string `db:"PRIVILEGE_TYPE"`
	Grantee       string `db:"GRANTEE"`
}

// GetPermission maps the row's PRIVILEGE_TYPE to a
// TablePolicyBindingPermission, returning UNSPECIFIED for unrecognized types.
func (r TablePermissionsRow) GetPermission() TablePolicyBindingPermission {
	switch r.PrivilegeType {
	case "SELECT":
		return TablePolicyBindingPermissionSelect
	case "INSERT":
		return TablePolicyBindingPermissionInsert
	case "UPDATE":
		return TablePolicyBindingPermissionUpdate
	case "DELETE":
		return TablePolicyBindingPermissionDelete
	default:
		return TablePolicyBindingPermissionUnspecified
	}
}

// Index is one flattened row of the INFORMATION_SCHEMA indexes/index_columns
// join queried by GetIndexes — one row per (index, column) pair, later merged
// into SpannerTableIndex values.
type Index struct {
	IndexName       string `db:"index_name"`
	IndexType       string `db:"index_type"`
	ColumnName      string `db:"column_name"`
	ColumnOrdering  string `db:"column_ordering"`
	IsUnique        bool   `db:"is_unique"`
	OrdinalPosition int    `db:"ordinal_position"`
}

// Constraint is one row of the INFORMATION_SCHEMA constraint join used to
// read foreign keys back from the database. The column tags are what the query
// scanner maps on, so they must keep matching the aliases the join selects.
type Constraint struct {
	ConstraintName    string `db:"CONSTRAINT_NAME"`
	ConstraintType    string `db:"CONSTRAINT_TYPE"`
	ConstrainedTable  string `db:"CONSTRAINED_TABLE"`
	ConstrainedColumn string `db:"CONSTRAINED_COLUMN"`
	UpdateRule        string `db:"UPDATE_RULE"`
	DeleteRule        string `db:"DELETE_RULE"`
	ReferencedTable   string `db:"REFERENCED_TABLE"`
	ReferencedColumn  string `db:"REFERENCED_COLUMN"`
}

// SequenceRow is one row of INFORMATION_SCHEMA.SEQUENCES left-joined with
// SEQUENCE_OPTIONS — one row per (sequence, option) pair.
type SequenceRow struct {
	Catalog      string `db:"CATALOG"`
	Schema       string `db:"SCHEMA"`
	SequenceName string `db:"SEQUENCE_NAME"`
	DataType     string `db:"DATA_TYPE"`

	// Pointers handle the potential NULLs from the LEFT JOIN
	OptionName  *string `db:"OPTION_NAME"`
	OptionValue *string `db:"OPTION_VALUE"`
	OptionType  *string `db:"OPTION_TYPE"`
}
