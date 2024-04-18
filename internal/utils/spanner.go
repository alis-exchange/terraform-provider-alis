package utils

import pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"

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
	SpannerTableDataType_NUMERIC
	SpannerTableDataType_JSON
)

func (t SpannerTableDataType) String() string {
	return [...]string{"BOOL", "INT64", "FLOAT64", "STRING", "BYTES", "DATE", "TIMESTAMP", "NUMERIC", "JSON"}[t-1]
}
func (t SpannerTableDataType) Proto() pb.SpannerTable_Schema_Column_SchemaType {
	return pb.SpannerTable_Schema_Column_SchemaType(t)
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
	SpannerTableDataType_NUMERIC.String(),
	SpannerTableDataType_JSON.String(),
}
