package schema

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

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

func (c *SpannerTableColumn) GetName() string {
	if c == nil {
		return ""
	}

	return c.Name
}

func (c *SpannerTableColumn) GetIsPrimaryKey() *wrapperspb.BoolValue {
	if c == nil {
		return nil
	}

	return c.IsPrimaryKey
}

func (c *SpannerTableColumn) GetIsComputed() *wrapperspb.BoolValue {
	if c == nil {
		return nil
	}

	return c.IsComputed
}

func (c *SpannerTableColumn) GetComputationDdl() *wrapperspb.StringValue {
	if c == nil {
		return nil
	}

	return c.ComputationDdl
}

func (c *SpannerTableColumn) GetAutoCreateTime() *wrapperspb.BoolValue {
	if c == nil {
		return nil
	}

	return c.AutoCreateTime
}

func (c *SpannerTableColumn) GetAutoUpdateTime() *wrapperspb.BoolValue {
	if c == nil {
		return nil
	}

	return c.AutoUpdateTime
}

func (c *SpannerTableColumn) GetType() string {
	if c == nil {
		return ""
	}

	return c.Type
}

func (c *SpannerTableColumn) GetSize() *wrapperspb.Int64Value {
	if c == nil {
		return nil
	}

	return c.Size
}

func (c *SpannerTableColumn) GetRequired() *wrapperspb.BoolValue {
	if c == nil {
		return nil
	}

	return c.Required
}

func (c *SpannerTableColumn) GetDefaultValue() *wrapperspb.StringValue {
	if c == nil {
		return nil
	}

	return c.DefaultValue
}

func (c *SpannerTableColumn) GetProtoFileDescriptorSet() *ProtoFileDescriptorSet {
	if c == nil {
		return nil
	}

	return c.ProtoFileDescriptorSet
}

// PrimaryKey returns true if the column is a primary key.
func (c *SpannerTableColumn) PrimaryKey() bool {
	return c.GetIsPrimaryKey() != nil && c.GetIsPrimaryKey().GetValue()
}

func (c *SpannerTableColumn) ddl() (string, error) {

	// Create DDL
	ddl := fmt.Sprintf("`%s`", c.GetName())
	var options []string

	// Set Type
	{
		if c.GetType() == SpannerTableDataTypeProto.String() {
			// Ensure proto package is set
			if c.GetProtoFileDescriptorSet() == nil ||
				c.GetProtoFileDescriptorSet().GetProtoPackage() == nil ||
				c.GetProtoFileDescriptorSet().GetProtoPackage().GetValue() == "" {
				return "", fmt.Errorf("proto_package is required for proto column %s", c.GetName())
			}

			ddl += fmt.Sprintf(" `%s`", c.GetProtoFileDescriptorSet().GetProtoPackage().GetValue())
		} else {
			ddl += fmt.Sprintf(" %s", c.GetType())
		}
	}

	// Set Size
	{
		size := "MAX"
		if c.GetSize() != nil {
			size = fmt.Sprintf("%d", c.GetSize().GetValue())
		}
		if c.GetType() == SpannerTableDataTypeString.String() || c.GetType() == SpannerTableDataTypeBytes.String() {
			ddl += fmt.Sprintf("(%s)", size)
		}
		if c.GetType() == SpannerTableDataTypeStringArray.String() {
			ddl = strings.TrimSuffix(ddl, ">")
			ddl += fmt.Sprintf("(%s)>", size)
		}
	}

	// Set Nullable
	{
		if c.GetRequired() != nil && c.GetRequired().GetValue() {
			ddl += " NOT NULL"
		}
	}

	// Set Computation DDL
	{

		if c.GetIsComputed() != nil && c.GetIsComputed().GetValue() {
			if c.GetComputationDdl() == nil || c.GetComputationDdl().GetValue() == "" {
				return "", fmt.Errorf("computation_ddl is required for computed column %s", c.GetName())
			}

			ddl += fmt.Sprintf(" AS (%s)", c.GetComputationDdl().GetValue())
		}

	}

	// Set Default Value
	{
		if c.GetDefaultValue() != nil {
			ddl += fmt.Sprintf(" DEFAULT (%s)", c.GetDefaultValue().GetValue())
		}
	}

	// Set auto update time
	{
		if c.Type == SpannerTableDataTypeTimestamp.String() && c.GetAutoUpdateTime() != nil {
			if c.GetAutoUpdateTime().GetValue() {
				options = append(options, "allow_commit_timestamp=true")
			} else {
				options = append(options, "allow_commit_timestamp=false")
			}
		}
	}

	if len(options) > 0 {
		ddl += " OPTIONS (" + strings.Join(options, ", ") + ")"
	}

	return ddl, nil
}

func (c *SpannerTableColumn) alterDdl(existingColumn *SpannerTableColumn) ([]string, error) {
	// There's only a handful of things that can be altered
	// 1. Nullable
	// 2. Default Value
	// 3. Size
	// These are the only ones that we'll check for

	var ddls []string

	{
		ddl := fmt.Sprintf("`%s`", c.GetName())
		ddlUpdated := false

		// Set Type
		{
			if c.GetType() == SpannerTableDataTypeProto.String() {
				// Ensure proto package is set
				if c.GetProtoFileDescriptorSet() == nil ||
					c.GetProtoFileDescriptorSet().GetProtoPackage() == nil ||
					c.GetProtoFileDescriptorSet().GetProtoPackage().GetValue() == "" {
					return nil, fmt.Errorf("proto_package is required for proto column %s", c.GetName())
				}

				ddl += fmt.Sprintf(" `%s`", c.GetProtoFileDescriptorSet().GetProtoPackage().GetValue())
			} else {
				ddl += fmt.Sprintf(" %s", c.GetType())
			}
		}

		// Handle Size
		{
			if c.GetType() == SpannerTableDataTypeString.String() || c.GetType() == SpannerTableDataTypeBytes.String() || c.GetType() == SpannerTableDataTypeStringArray.String() {
				// If the existing column has a size and the new column does not
				if existingColumn.GetSize() != nil && existingColumn.GetSize().GetValue() > 0 && (c.GetSize() == nil || c.GetSize().GetValue() == 0) {
					size := "MAX"

					switch c.GetType() {
					case SpannerTableDataTypeString.String(), SpannerTableDataTypeBytes.String():
						ddl += fmt.Sprintf("(%s)", size)
						ddlUpdated = true
					case SpannerTableDataTypeStringArray.String():
						ddl = strings.TrimSuffix(ddl, ">")
						ddl += fmt.Sprintf("(%s)>", size)
						ddlUpdated = true
					}
				}

				// If the existing column does not have a size and the new column does
				if (existingColumn.GetSize() == nil || existingColumn.GetSize().GetValue() == 0) && c.GetSize() != nil && c.GetSize().GetValue() > 0 {
					size := fmt.Sprintf("%d", c.GetSize().GetValue())

					switch c.GetType() {
					case SpannerTableDataTypeString.String(), SpannerTableDataTypeBytes.String():
						ddl += fmt.Sprintf("(%s)", size)
						ddlUpdated = true
					case SpannerTableDataTypeStringArray.String():
						ddl = strings.TrimSuffix(ddl, ">")
						ddl += fmt.Sprintf("(%s)>", size)
						ddlUpdated = true
					}
				}

				// If the existing column has a size and the new column has a different size
				if existingColumn.GetSize() != nil && existingColumn.GetSize().GetValue() > 0 &&
					c.GetSize() != nil && c.GetSize().GetValue() > 0 &&
					existingColumn.GetSize().GetValue() != c.GetSize().GetValue() {
					size := fmt.Sprintf("%d", c.GetSize().GetValue())

					switch c.GetType() {
					case SpannerTableDataTypeString.String(), SpannerTableDataTypeBytes.String():
						ddl += fmt.Sprintf("(%s)", size)
						ddlUpdated = true
					case SpannerTableDataTypeStringArray.String():
						ddl = strings.TrimSuffix(ddl, ">")
						ddl += fmt.Sprintf("(%s)>", size)
						ddlUpdated = true
					}
				}
			}
		}

		// Handle Nullable
		{
			// If the existing column is nullable and the new column is not
			if (existingColumn.GetRequired() == nil || !existingColumn.GetRequired().GetValue()) && c.GetRequired() != nil && c.GetRequired().GetValue() {
				ddlUpdated = true
			}

			// If the existing column is not nullable and the new column is
			if existingColumn.GetRequired() != nil && existingColumn.GetRequired().GetValue() && (c.GetRequired() == nil || !c.GetRequired().GetValue()) {
				ddl += " NOT NULL"
				ddlUpdated = true
			}
		}

		if ddlUpdated {
			ddls = append(ddls, ddl)
		}
	}

	{
		ddl := fmt.Sprintf("`%s`", c.GetName())
		ddlUpdated := false

		// Handle Default Value
		{

			// If the existing column has a default value and the new column does not
			if existingColumn.GetDefaultValue() != nil && existingColumn.GetDefaultValue().GetValue() != "" && (c.GetDefaultValue() == nil || c.GetDefaultValue().GetValue() == "") {
				ddl += " DROP DEFAULT"
				ddlUpdated = true
			}

			// If the existing column does not have a default value and the new column does
			if (existingColumn.GetDefaultValue() == nil || existingColumn.GetDefaultValue().GetValue() == "") && c.GetDefaultValue() != nil && c.GetDefaultValue().GetValue() != "" {
				ddl += fmt.Sprintf(" SET DEFAULT (%s)", c.GetDefaultValue().GetValue())
				ddlUpdated = true
			}

			// If the existing column has a default value and the new column has a different default value
			if existingColumn.GetDefaultValue() != nil && existingColumn.GetDefaultValue().GetValue() != "" &&
				c.GetDefaultValue() != nil && c.GetDefaultValue().GetValue() != "" &&
				existingColumn.GetDefaultValue().GetValue() != c.GetDefaultValue().GetValue() {
				ddl += fmt.Sprintf(" SET DEFAULT (%s)", c.GetDefaultValue().GetValue())
				ddlUpdated = true
			}
		}

		if ddlUpdated {
			ddls = append(ddls, ddl)
		}
	}

	return ddls, nil
}

func (c *SpannerTableColumn) compare(other *SpannerTableColumn) bool {
	if c == nil && other == nil {
		return true
	}

	if c == nil || other == nil {
		return false
	}

	if c.GetName() != other.GetName() {
		return false
	}

	if c.Type != other.Type {
		return false
	}

	if c.PrimaryKey() != other.PrimaryKey() {
		return false
	}

	if c.GetIsComputed().GetValue() != other.GetIsComputed().GetValue() {
		return false
	}

	if c.GetComputationDdl().GetValue() != other.GetComputationDdl().GetValue() {
		return false
	}

	if c.GetAutoCreateTime().GetValue() != other.GetAutoCreateTime().GetValue() {
		return false
	}

	if c.GetAutoUpdateTime().GetValue() != other.GetAutoUpdateTime().GetValue() {
		return false
	}

	if c.GetSize().GetValue() != other.GetSize().GetValue() {
		return false
	}

	if c.GetRequired().GetValue() != other.GetRequired().GetValue() {
		return false
	}

	if c.GetDefaultValue().GetValue() != other.GetDefaultValue().GetValue() {
		return false
	}

	if !c.GetProtoFileDescriptorSet().compare(other.GetProtoFileDescriptorSet()) {
		return false
	}

	return true

}
