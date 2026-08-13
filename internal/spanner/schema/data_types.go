package schema

// SpannerTableDataType is a type for Spanner table column data types.
type SpannerTableDataType int64

const (
	SpannerTableDataTypeBool SpannerTableDataType = iota + 1
	SpannerTableDataTypeInt64
	SpannerTableDataTypeFloat64
	SpannerTableDataTypeString
	SpannerTableDataTypeBytes
	SpannerTableDataTypeDate
	SpannerTableDataTypeTimestamp
	SpannerTableDataTypeJson
	SpannerTableDataTypeProto
	SpannerTableDataTypeStringArray
	SpannerTableDataTypeInt64Array
	SpannerTableDataTypeFloat32Array
	SpannerTableDataTypeFloat64Array
)

// String returns the DDL spelling of the type. Values outside the declared
// constants, the zero value included, render as "" — a String method that
// panics would take the whole provider down from a log line.
func (t SpannerTableDataType) String() string {
	names := [...]string{
		"BOOL", "INT64", "FLOAT64", "STRING", "BYTES", "DATE", "TIMESTAMP", "JSON", "PROTO",
		"ARRAY<STRING>", "ARRAY<INT64>", "ARRAY<FLOAT32>", "ARRAY<FLOAT64>",
	}
	if t < 1 || int(t) > len(names) {
		return ""
	}

	return names[t-1]
}

// SpannerTableDataTypes is a list of all Spanner table column data types.
var SpannerTableDataTypes = []string{
	SpannerTableDataTypeBool.String(),
	SpannerTableDataTypeInt64.String(),
	SpannerTableDataTypeFloat64.String(),
	SpannerTableDataTypeString.String(),
	SpannerTableDataTypeBytes.String(),
	SpannerTableDataTypeDate.String(),
	SpannerTableDataTypeTimestamp.String(),
	SpannerTableDataTypeJson.String(),
	SpannerTableDataTypeProto.String(),
	SpannerTableDataTypeStringArray.String(),
	SpannerTableDataTypeInt64Array.String(),
	SpannerTableDataTypeFloat32Array.String(),
	SpannerTableDataTypeFloat64Array.String(),
}
