package spanner

import (
	"context"
	"testing"

	tableschema "terraform-provider-alis/internal/spanner/schema"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// fullColumnModel exercises every attribute of the TF column model.
func fullColumnModel() spannerTableColumn {
	return spannerTableColumn{
		Name:           types.StringValue("proto_col"),
		IsPrimaryKey:   types.BoolValue(true),
		IsComputed:     types.BoolValue(true),
		ComputationDdl: types.StringValue("CONCAT(a, b)"),
		IsStored:       types.BoolValue(true),
		AutoUpdateTime: types.BoolValue(false),
		Type:           types.StringValue("PROTO"),
		Size:           types.Int64Value(255),
		Required:       types.BoolValue(true),
		DefaultValue:   types.StringValue("'x'"),
		ProtoPackage:   types.StringValue("com.example.Msg"),
	}
}

// minimalColumnModel sets only the required attributes; everything else stays null.
func minimalColumnModel() spannerTableColumn {
	return spannerTableColumn{
		Name: types.StringValue("email"),
		Type: types.StringValue("STRING(MAX)"),
	}
}

func columnList(t *testing.T, cols []spannerTableColumn) types.List {
	t.Helper()
	list, d := types.ListValueFrom(context.Background(), types.ObjectType{
		AttrTypes: spannerTableColumn{}.attrTypes(),
	}, cols)
	if d.HasError() {
		t.Fatalf("building column list: %v", d)
	}
	return list
}

func TestTableColumnsToSchema(t *testing.T) {
	ctx := context.Background()

	got, d := tableColumnsToSchema(ctx, columnList(t, []spannerTableColumn{fullColumnModel(), minimalColumnModel()}))
	if d.HasError() {
		t.Fatalf("tableColumnsToSchema: %v", d)
	}
	if len(got) != 2 {
		t.Fatalf("got %d columns, want 2", len(got))
	}

	full := got[0]
	if full.Name != "proto_col" || full.Type != "PROTO" {
		t.Errorf("full column identity = (%q, %q), want (proto_col, PROTO)", full.Name, full.Type)
	}
	if !full.GetIsPrimaryKey().GetValue() || !full.GetIsComputed().GetValue() || !full.GetIsStored().GetValue() {
		t.Errorf("full column booleans lost: pk=%v computed=%v stored=%v",
			full.GetIsPrimaryKey().GetValue(), full.GetIsComputed().GetValue(), full.GetIsStored().GetValue())
	}
	if full.GetComputationDdl().GetValue() != "CONCAT(a, b)" || full.GetSize().GetValue() != 255 ||
		!full.GetRequired().GetValue() || full.GetDefaultValue().GetValue() != "'x'" {
		t.Errorf("full column attributes lost: ddl=%q size=%d required=%v default=%q",
			full.GetComputationDdl().GetValue(), full.GetSize().GetValue(),
			full.GetRequired().GetValue(), full.GetDefaultValue().GetValue())
	}
	// AutoUpdateTime false must survive as an explicit false wrapper, not nil.
	if full.AutoUpdateTime == nil || full.AutoUpdateTime.GetValue() {
		t.Errorf("AutoUpdateTime = %v, want explicit false", full.AutoUpdateTime)
	}
	if full.GetProtoPackage().GetValue() != "com.example.Msg" {
		t.Errorf("proto package lost: %v", full.ProtoPackage)
	}

	minimal := got[1]
	if minimal.Name != "email" || minimal.Type != "STRING(MAX)" {
		t.Errorf("minimal identity = (%q, %q)", minimal.Name, minimal.Type)
	}
	// Null model attributes must stay nil wrappers (absent), not become explicit false/zero.
	if minimal.IsPrimaryKey != nil || minimal.IsComputed != nil || minimal.IsStored != nil ||
		minimal.AutoUpdateTime != nil || minimal.Size != nil || minimal.Required != nil ||
		minimal.DefaultValue != nil || minimal.ProtoPackage != nil {
		t.Errorf("minimal column grew explicit values: %+v", minimal)
	}
}

func TestTableColumnsToSchema_NullList(t *testing.T) {
	got, d := tableColumnsToSchema(context.Background(), types.ListNull(types.ObjectType{
		AttrTypes: spannerTableColumn{}.attrTypes(),
	}))
	if d.HasError() || got != nil {
		t.Errorf("null list should convert to nil columns without diagnostics, got %v / %v", got, d)
	}
}

// Round-trip: model list → schema → model list must be identical, nulls included.
func TestTableColumnsRoundTrip(t *testing.T) {
	ctx := context.Background()
	original := columnList(t, []spannerTableColumn{fullColumnModel(), minimalColumnModel()})

	schemaCols, d := tableColumnsToSchema(ctx, original)
	if d.HasError() {
		t.Fatalf("to schema: %v", d)
	}
	back, d := tableColumnsToModel(ctx, schemaCols)
	if d.HasError() {
		t.Fatalf("to model: %v", d)
	}
	if !original.Equal(back) {
		t.Errorf("round-trip drift:\noriginal: %s\nback:     %s", original, back)
	}
}

func TestTableInterleaveRoundTrip(t *testing.T) {
	model := &spannerTableInterleave{
		ParentTable: types.StringValue("parents"),
		OnDelete:    types.StringValue("CASCADE"),
	}

	sch := tableInterleaveToSchema(model)
	if sch.ParentTable != "parents" || sch.OnDelete != tableschema.SpannerTableConstraintActionCascade {
		t.Fatalf("to schema: %+v", sch)
	}

	back := tableInterleaveToModel(sch)
	if !back.ParentTable.Equal(model.ParentTable) || !back.OnDelete.Equal(model.OnDelete) {
		t.Errorf("round-trip drift: %+v vs %+v", back, model)
	}

	if tableInterleaveToSchema(nil) != nil || tableInterleaveToModel(nil) != nil {
		t.Error("nil interleave must stay nil in both directions")
	}
}
