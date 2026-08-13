package spanner

import (
	"context"

	tableschema "terraform-provider-alis/internal/spanner/schema"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// tableColumnsToSchema converts the Terraform column list into schema columns.
// Null model attributes stay absent (nil wrappers) so the schema layer can
// distinguish "not configured" from an explicit false/zero.
func tableColumnsToSchema(ctx context.Context, list types.List) ([]*tableschema.SpannerTableColumn, diag.Diagnostics) {
	var diags diag.Diagnostics

	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	columns := make([]spannerTableColumn, 0, len(list.Elements()))
	diags.Append(list.ElementsAs(ctx, &columns, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]*tableschema.SpannerTableColumn, 0, len(columns))
	for _, column := range columns {
		col := &tableschema.SpannerTableColumn{}

		if !column.Name.IsNull() {
			col.Name = column.Name.ValueString()
		}
		if !column.IsPrimaryKey.IsNull() {
			col.IsPrimaryKey = wrapperspb.Bool(column.IsPrimaryKey.ValueBool())
		}
		if !column.IsComputed.IsNull() {
			col.IsComputed = wrapperspb.Bool(column.IsComputed.ValueBool())
		}
		if !column.ComputationDdl.IsNull() {
			col.ComputationDdl = wrapperspb.String(column.ComputationDdl.ValueString())
		}
		if !column.IsStored.IsNull() {
			col.IsStored = wrapperspb.Bool(column.IsStored.ValueBool())
		}
		if !column.AutoUpdateTime.IsNull() {
			col.AutoUpdateTime = wrapperspb.Bool(column.AutoUpdateTime.ValueBool())
		}
		if !column.Type.IsNull() {
			col.Type = column.Type.ValueString()
		}
		if !column.Size.IsNull() {
			col.Size = wrapperspb.Int64(column.Size.ValueInt64())
		}
		if !column.Required.IsNull() {
			col.Required = wrapperspb.Bool(column.Required.ValueBool())
		}
		if !column.DefaultValue.IsNull() {
			col.DefaultValue = wrapperspb.String(column.DefaultValue.ValueString())
		}

		if !column.ProtoPackage.IsNull() {
			col.ProtoPackage = wrapperspb.String(column.ProtoPackage.ValueString())
		}

		result = append(result, col)
	}

	return result, diags
}

// tableColumnsToModel converts schema columns back into the Terraform column list.
// It is the exact inverse of tableColumnsToSchema: absent (nil) schema attributes
// become null model attributes.
func tableColumnsToModel(ctx context.Context, columns []*tableschema.SpannerTableColumn) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	objectType := types.ObjectType{AttrTypes: spannerTableColumn{}.attrTypes()}

	if columns == nil {
		return types.ListNull(objectType), diags
	}

	cols := make([]*spannerTableColumn, 0, len(columns))
	for _, column := range columns {
		col := &spannerTableColumn{
			Name: types.StringValue(column.Name),
		}

		if column.IsPrimaryKey != nil {
			col.IsPrimaryKey = types.BoolValue(column.IsPrimaryKey.GetValue())
		}
		if column.IsComputed != nil {
			col.IsComputed = types.BoolValue(column.IsComputed.GetValue())
		}
		if column.ComputationDdl != nil {
			col.ComputationDdl = types.StringValue(column.ComputationDdl.GetValue())
		}
		if column.IsStored != nil {
			col.IsStored = types.BoolValue(column.IsStored.GetValue())
		}
		if column.AutoUpdateTime != nil {
			col.AutoUpdateTime = types.BoolValue(column.AutoUpdateTime.GetValue())
		}
		if column.Type != "" {
			col.Type = types.StringValue(column.Type)
		}
		if column.Size != nil {
			col.Size = types.Int64Value(column.Size.GetValue())
		}
		if column.Required != nil {
			col.Required = types.BoolValue(column.Required.GetValue())
		}
		if column.DefaultValue != nil {
			col.DefaultValue = types.StringValue(column.DefaultValue.GetValue())
		}
		if column.ProtoPackage != nil {
			col.ProtoPackage = types.StringValue(column.ProtoPackage.GetValue())
		}

		cols = append(cols, col)
	}

	list, d := types.ListValueFrom(ctx, objectType, cols)
	diags.Append(d...)
	return list, diags
}

// tableInterleaveToSchema converts the Terraform interleave block; nil stays nil.
func tableInterleaveToSchema(interleave *spannerTableInterleave) *tableschema.SpannerTableInterleave {
	if interleave == nil {
		return nil
	}

	out := &tableschema.SpannerTableInterleave{
		ParentTable: interleave.ParentTable.ValueString(),
	}
	if !interleave.OnDelete.IsNull() {
		out.OnDelete = tableschema.SpannerTableConstraintActionFromString(interleave.OnDelete.ValueString())
	}
	return out
}

// tableInterleaveToModel converts a schema interleave back to the Terraform block; nil stays nil.
func tableInterleaveToModel(interleave *tableschema.SpannerTableInterleave) *spannerTableInterleave {
	if interleave == nil {
		return nil
	}

	out := &spannerTableInterleave{
		ParentTable: types.StringValue(interleave.ParentTable),
	}
	if interleave.OnDelete != tableschema.SpannerTableConstraintActionUnspecified {
		out.OnDelete = types.StringValue(interleave.OnDelete.String())
	}
	return out
}
