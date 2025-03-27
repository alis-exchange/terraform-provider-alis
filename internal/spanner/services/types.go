package services

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

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

type SpannerTableRowDeletionPolicy struct {
	// The name of the TIMESTAMP column that is used to determine when a row is deleted
	Column string
	// The duration after which a row is deleted in days
	Duration *wrapperspb.Int64Value
}
