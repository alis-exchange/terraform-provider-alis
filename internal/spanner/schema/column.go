package schema

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// PreserveUnsetBooleans collapses hydrated boolean attributes back to unset
// where the prior state left them unset and hydration answered "false".
// INFORMATION_SCHEMA always answers explicitly, but an explicit false and an
// absent boolean produce identical DDL, so preferring the prior shape keeps
// refresh from flagging phantom diffs on attributes the config omitted.
// Hydrated true always survives: that is real drift.
func PreserveUnsetBooleans(prior, hydrated []*SpannerTableColumn) {
	priorByName := make(map[string]*SpannerTableColumn, len(prior))
	for _, c := range prior {
		priorByName[c.Name] = c
	}

	collapse := func(priorV, hydratedV *wrapperspb.BoolValue) *wrapperspb.BoolValue {
		if priorV == nil && !hydratedV.GetValue() {
			return nil
		}
		return hydratedV
	}

	for _, h := range hydrated {
		p, ok := priorByName[h.Name]
		if !ok {
			continue
		}
		h.IsPrimaryKey = collapse(p.IsPrimaryKey, h.IsPrimaryKey)
		h.IsComputed = collapse(p.IsComputed, h.IsComputed)
		h.IsStored = collapse(p.IsStored, h.IsStored)
		h.AutoUpdateTime = collapse(p.AutoUpdateTime, h.AutoUpdateTime)
		h.Required = collapse(p.Required, h.Required)
	}
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
	// Whether the generated column is stored
	// This is only valid for computed columns
	IsStored *wrapperspb.BoolValue
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
	// The fully-qualified proto message name for the column.
	//
	// Required for PROTO columns. The proto bundle carrying the message must
	// already exist in the database; the provider does not manage bundles.
	ProtoPackage *wrapperspb.StringValue
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

func (c *SpannerTableColumn) GetIsStored() *wrapperspb.BoolValue {
	if c == nil {
		return nil
	}

	return c.IsStored
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

func (c *SpannerTableColumn) GetProtoPackage() *wrapperspb.StringValue {
	if c == nil {
		return nil
	}

	return c.ProtoPackage
}

// PrimaryKey returns true if the column is a primary key.
func (c *SpannerTableColumn) PrimaryKey() bool {
	return c.GetIsPrimaryKey() != nil && c.GetIsPrimaryKey().GetValue()
}

// ddl renders the column definition fragment shared by CREATE TABLE and ADD
// COLUMN: name, type, size, NOT NULL, generation expression, DEFAULT, and
// OPTIONS. Proto columns render the backticked fully-qualified message name
// and error without a ProtoPackage; computed columns error without a
// ComputationDdl.
func (c *SpannerTableColumn) ddl() (string, error) {
	// Create DDL
	ddl := fmt.Sprintf("`%s`", c.GetName())
	var options []string

	// Set Type
	{
		if c.GetType() == SpannerTableDataTypeProto.String() {
			// Ensure proto package is set
			if c.GetProtoPackage().GetValue() == "" {
				return "", fmt.Errorf("proto_package is required for proto column %s", c.GetName())
			}

			ddl += fmt.Sprintf(" `%s`", c.GetProtoPackage().GetValue())
		} else {
			ddl += " " + c.GetType()
		}
	}

	// Set Size
	{
		size := "MAX"
		if c.GetSize() != nil {
			size = strconv.FormatInt(c.GetSize().GetValue(), 10)
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
			if c.GetIsStored() != nil && c.GetIsStored().GetValue() {
				ddl += " STORED"
			}
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

// alterDdl renders the ALTER COLUMN fragments needed to move existingColumn
// to this column's shape. Only size, nullability, and the default value can
// change in place — anything else requires a table replace (see
// ClassifyColumnChange). The type/NOT NULL form and the SET/DROP DEFAULT form
// are distinct ALTER COLUMN productions in Spanner DDL, so default-value
// changes render as a separate fragment.
func (c *SpannerTableColumn) alterDdl(existingColumn *SpannerTableColumn) ([]string, error) {
	var ddls []string

	{
		ddl := fmt.Sprintf("`%s`", c.GetName())
		ddlUpdated := false

		// Set Type
		{
			if c.GetType() == SpannerTableDataTypeProto.String() {
				// Ensure proto package is set
				if c.GetProtoPackage().GetValue() == "" {
					return nil, fmt.Errorf("proto_package is required for proto column %s", c.GetName())
				}

				ddl += fmt.Sprintf(" `%s`", c.GetProtoPackage().GetValue())
			} else {
				ddl += " " + c.GetType()
			}
		}

		// Handle Size
		{
			if c.GetType() == SpannerTableDataTypeString.String() || c.GetType() == SpannerTableDataTypeBytes.String() ||
				c.GetType() == SpannerTableDataTypeStringArray.String() {
				// If the existing column has a size and the new column does not
				if existingColumn.GetSize() != nil && existingColumn.GetSize().GetValue() > 0 &&
					(c.GetSize() == nil || c.GetSize().GetValue() == 0) {
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
				if (existingColumn.GetSize() == nil || existingColumn.GetSize().GetValue() == 0) && c.GetSize() != nil &&
					c.GetSize().GetValue() > 0 {
					size := strconv.FormatInt(c.GetSize().GetValue(), 10)

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
					size := strconv.FormatInt(c.GetSize().GetValue(), 10)

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
			if (existingColumn.GetRequired() == nil || !existingColumn.GetRequired().GetValue()) && c.GetRequired() != nil &&
				c.GetRequired().GetValue() {
				ddlUpdated = true
			}

			// If the existing column is not nullable and the new column is
			if existingColumn.GetRequired() != nil && existingColumn.GetRequired().GetValue() &&
				(c.GetRequired() == nil || !c.GetRequired().GetValue()) {
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
			if existingColumn.GetDefaultValue() != nil && existingColumn.GetDefaultValue().GetValue() != "" &&
				(c.GetDefaultValue() == nil || c.GetDefaultValue().GetValue() == "") {
				ddl += " DROP DEFAULT"
				ddlUpdated = true
			}

			// If the existing column does not have a default value and the new column does
			if (existingColumn.GetDefaultValue() == nil || existingColumn.GetDefaultValue().GetValue() == "") &&
				c.GetDefaultValue() != nil &&
				c.GetDefaultValue().GetValue() != "" {
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

// compare reports whether two columns are semantically identical. Wrapper
// fields are compared by value, so an unset boolean equals an explicit false
// — the two shapes produce identical DDL.
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

	if c.GetIsStored().GetValue() != other.GetIsStored().GetValue() {
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

	if c.GetProtoPackage().GetValue() != other.GetProtoPackage().GetValue() {
		return false
	}

	return true
}
