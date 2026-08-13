package schema

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/connfake"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

// seedProbeTable stubs the fake with the INFORMATION_SCHEMA shapes of a table
// exercising every modeled column attribute (mirrors the emulator coverage
// probe in internal/spanner/conn/emulator_coverage_test.go).
func seedProbeTable(fake *connfake.Fake) {
	fake.OnQuery("FROM INFORMATION_SCHEMA.TABLES", []tableInfoRow{
		{TableName: ns("probe")},
	})
	fake.OnQuery("FROM INFORMATION_SCHEMA.COLUMNS", []*informationSchemaColumnRow{
		{ColumnName: ns("id"), SpannerType: ns("INT64"), IsNullable: ns("NO"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("sub_id"), SpannerType: ns("STRING(64)"), IsNullable: ns("NO"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("str_max"), SpannerType: ns("STRING(MAX)"), IsNullable: ns("YES"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("flt"), SpannerType: ns("FLOAT64"), IsNullable: ns("NO"), ColumnDefault: ns("0.0"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("label"), SpannerType: ns("STRING(50)"), IsNullable: ns("YES"), ColumnDefault: ns("'hello'"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("update_time"), SpannerType: ns("TIMESTAMP"), IsNullable: ns("YES"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("off_time"), SpannerType: ns("TIMESTAMP"), IsNullable: ns("YES"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("created_at"), SpannerType: ns("TIMESTAMP"), IsNullable: ns("YES"), ColumnDefault: ns("CURRENT_TIMESTAMP()"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("gen_stored"), SpannerType: ns("INT64"), IsNullable: ns("YES"), IsGenerated: ns("ALWAYS"), IsStored: ns("YES"), GenerationExpr: ns("cnt + 1")},
		{ColumnName: ns("gen_virtual"), SpannerType: ns("INT64"), IsNullable: ns("YES"), IsGenerated: ns("ALWAYS"), IsStored: ns("NO"), GenerationExpr: ns("cnt + 2")},
		{ColumnName: ns("tags"), SpannerType: ns("ARRAY<STRING(100)>"), IsNullable: ns("YES"), IsGenerated: ns("NEVER")},
		{ColumnName: ns("proto_col"), SpannerType: ns("`tftest.Simple`"), IsNullable: ns("YES"), IsGenerated: ns("NEVER")},
	})
	fake.OnQuery("INFORMATION_SCHEMA.INDEX_COLUMNS", []*primaryKeyRow{
		{ColumnName: ns("id")},
		{ColumnName: ns("sub_id")},
	})
	fake.OnQuery("INFORMATION_SCHEMA.COLUMN_OPTIONS", []*columnOptionRow{
		{ColumnName: ns("update_time"), OptionName: ns("allow_commit_timestamp"), OptionValue: ns("TRUE")},
		{ColumnName: ns("off_time"), OptionName: ns("allow_commit_timestamp"), OptionValue: ns("FALSE")},
	})
}

func TestSpannerTable_Get_HydratesFromInformationSchemaOnly(t *testing.T) {
	fake := connfake.New()
	seedProbeTable(fake)

	got, err := (&SpannerTable{}).Get(context.Background(), fake,
		"projects/p/instances/i/databases/d/tables/probe")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Exactly the four INFORMATION_SCHEMA queries — no other source.
	if ops := fake.OpsOf(connfake.OpQuery); len(ops) != 4 {
		for _, op := range ops {
			t.Logf("query: %s", op.SQL)
		}
		t.Errorf("Get() issued %d queries, want 4 (TABLES, COLUMNS, INDEX_COLUMNS, COLUMN_OPTIONS)", len(ops))
	}
	// Reading must never change schema or data.
	if n := len(fake.OpsOf(connfake.OpExecuteDDL)) + len(fake.OpsOf(connfake.OpExec)); n != 0 {
		t.Errorf("Get() issued %d DDL/DML ops, want 0", n)
	}

	if got.Interleave != nil {
		t.Errorf("Interleave = %+v, want nil for a root table", got.Interleave)
	}

	want := map[string]*SpannerTableColumn{
		"id":          {Name: "id", Type: "INT64", Required: wrapperspb.Bool(true), IsComputed: wrapperspb.Bool(false), IsPrimaryKey: wrapperspb.Bool(true)},
		"sub_id":      {Name: "sub_id", Type: "STRING", Size: wrapperspb.Int64(64), Required: wrapperspb.Bool(true), IsComputed: wrapperspb.Bool(false), IsPrimaryKey: wrapperspb.Bool(true)},
		"str_max":     {Name: "str_max", Type: "STRING", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false)},
		"flt":         {Name: "flt", Type: "FLOAT64", Required: wrapperspb.Bool(true), DefaultValue: wrapperspb.String("0.0"), IsComputed: wrapperspb.Bool(false)},
		"label":       {Name: "label", Type: "STRING", Size: wrapperspb.Int64(50), Required: wrapperspb.Bool(false), DefaultValue: wrapperspb.String("'hello'"), IsComputed: wrapperspb.Bool(false)},
		"update_time": {Name: "update_time", Type: "TIMESTAMP", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false), AutoUpdateTime: wrapperspb.Bool(true)},
		"off_time":    {Name: "off_time", Type: "TIMESTAMP", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false), AutoUpdateTime: wrapperspb.Bool(false)},
		"created_at":  {Name: "created_at", Type: "TIMESTAMP", Required: wrapperspb.Bool(false), DefaultValue: wrapperspb.String("CURRENT_TIMESTAMP()"), IsComputed: wrapperspb.Bool(false)},
		"gen_stored":  {Name: "gen_stored", Type: "INT64", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(true), ComputationDdl: wrapperspb.String("cnt + 1"), IsStored: wrapperspb.Bool(true)},
		"gen_virtual": {Name: "gen_virtual", Type: "INT64", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(true), ComputationDdl: wrapperspb.String("cnt + 2"), IsStored: wrapperspb.Bool(false)},
		"tags":        {Name: "tags", Type: "ARRAY<STRING>", Size: wrapperspb.Int64(100), Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false)},
		"proto_col": {Name: "proto_col", Type: "PROTO", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false),
			ProtoPackage: wrapperspb.String("tftest.Simple")},
	}

	if len(got.GetSchema().GetColumns()) != len(want) {
		t.Fatalf("got %d columns, want %d", len(got.GetSchema().GetColumns()), len(want))
	}
	for _, col := range got.GetSchema().GetColumns() {
		w, ok := want[col.Name]
		if !ok {
			t.Errorf("unexpected column %q", col.Name)
			continue
		}
		assertColumnEqual(t, col, w)
	}
}

func assertColumnEqual(t *testing.T, got, want *SpannerTableColumn) {
	t.Helper()
	if got.Type != want.Type {
		t.Errorf("%s: Type = %q, want %q", got.Name, got.Type, want.Type)
	}
	assertInt64Wrapper(t, got.Name, "Size", got.Size, want.Size)
	assertBoolWrapper(t, got.Name, "Required", got.Required, want.Required)
	assertBoolWrapper(t, got.Name, "IsPrimaryKey", got.IsPrimaryKey, want.IsPrimaryKey)
	assertBoolWrapper(t, got.Name, "IsComputed", got.IsComputed, want.IsComputed)
	assertBoolWrapper(t, got.Name, "IsStored", got.IsStored, want.IsStored)
	assertBoolWrapper(t, got.Name, "AutoUpdateTime", got.AutoUpdateTime, want.AutoUpdateTime)
	assertStringWrapper(t, got.Name, "DefaultValue", got.DefaultValue, want.DefaultValue)
	assertStringWrapper(t, got.Name, "ComputationDdl", got.ComputationDdl, want.ComputationDdl)
	assertStringWrapper(t, got.Name, "ProtoPackage", got.ProtoPackage, want.ProtoPackage)
}

func assertBoolWrapper(t *testing.T, col, field string, got, want *wrapperspb.BoolValue) {
	t.Helper()
	if (got == nil) != (want == nil) || got.GetValue() != want.GetValue() {
		t.Errorf("%s: %s = %v, want %v", col, field, got, want)
	}
}

func assertInt64Wrapper(t *testing.T, col, field string, got, want *wrapperspb.Int64Value) {
	t.Helper()
	if (got == nil) != (want == nil) || got.GetValue() != want.GetValue() {
		t.Errorf("%s: %s = %v, want %v", col, field, got, want)
	}
}

func assertStringWrapper(t *testing.T, col, field string, got, want *wrapperspb.StringValue) {
	t.Helper()
	if (got == nil) != (want == nil) || got.GetValue() != want.GetValue() {
		t.Errorf("%s: %s = %v, want %v", col, field, got, want)
	}
}

func TestSpannerTable_Get_Interleave(t *testing.T) {
	fake := connfake.New()
	fake.OnQuery("FROM INFORMATION_SCHEMA.TABLES", []tableInfoRow{
		{TableName: ns("probe_child"), ParentTableName: ns("probe"), OnDeleteAction: ns("CASCADE"), InterleaveType: ns("IN PARENT")},
	})
	fake.OnQuery("FROM INFORMATION_SCHEMA.COLUMNS", []*informationSchemaColumnRow{
		{ColumnName: ns("id"), SpannerType: ns("INT64"), IsNullable: ns("NO"), IsGenerated: ns("NEVER")},
	})

	got, err := (&SpannerTable{}).Get(context.Background(), fake,
		"projects/p/instances/i/databases/d/tables/probe_child")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Interleave == nil || got.Interleave.ParentTable != "probe" || got.Interleave.OnDelete != SpannerTableConstraintActionCascade {
		t.Errorf("Interleave = %+v, want parent probe with ON DELETE CASCADE", got.Interleave)
	}
}

func TestSpannerTable_Get_NotFound(t *testing.T) {
	fake := connfake.New()
	// No TABLES stub: single-row dest yields NotFound per the port contract.
	_, err := (&SpannerTable{}).Get(context.Background(), fake,
		"projects/p/instances/i/databases/d/tables/missing")
	var notFound ErrTableNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("Get() error = %v, want ErrTableNotFound", err)
	}
}

func TestPreserveUnsetBooleans(t *testing.T) {
	prior := []*SpannerTableColumn{
		{Name: "id", IsPrimaryKey: wrapperspb.Bool(true), Required: wrapperspb.Bool(true)},
		// display_name's config omitted every boolean.
		{Name: "display_name"},
		// explicit_false wrote required = false explicitly.
		{Name: "explicit_false", Required: wrapperspb.Bool(false)},
		{Name: "dropped"},
	}
	hydrated := []*SpannerTableColumn{
		{Name: "id", IsPrimaryKey: wrapperspb.Bool(true), Required: wrapperspb.Bool(true), IsComputed: wrapperspb.Bool(false)},
		{Name: "display_name", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false), IsStored: wrapperspb.Bool(false), AutoUpdateTime: wrapperspb.Bool(false), IsPrimaryKey: wrapperspb.Bool(false)},
		{Name: "explicit_false", Required: wrapperspb.Bool(false), IsComputed: wrapperspb.Bool(false)},
		// drifted turned computed on outside Terraform: explicit true survives.
		{Name: "drifted", IsComputed: wrapperspb.Bool(true)},
	}

	PreserveUnsetBooleans(prior, hydrated)

	byName := map[string]*SpannerTableColumn{}
	for _, c := range hydrated {
		byName[c.Name] = c
	}

	// Hydrated false collapses to unset only where prior state was unset.
	if c := byName["display_name"]; c.Required != nil || c.IsComputed != nil || c.IsStored != nil || c.AutoUpdateTime != nil || c.IsPrimaryKey != nil {
		t.Errorf("display_name: hydrated-false booleans should collapse to unset, got %+v", c)
	}
	// Explicit prior values are preserved as-is.
	if c := byName["id"]; c.IsPrimaryKey.GetValue() != true || c.Required.GetValue() != true {
		t.Errorf("id: explicit prior booleans must survive, got %+v", c)
	}
	if c := byName["id"]; c.IsComputed != nil {
		t.Errorf("id: IsComputed false with unset prior should collapse to unset, got %v", c.IsComputed)
	}
	if c := byName["explicit_false"]; c.Required == nil || c.Required.GetValue() != false {
		t.Errorf("explicit_false: explicitly-false prior must stay explicit, got %v", c.Required)
	}
	// True from hydration is real drift and must never be masked.
	if c := byName["drifted"]; c.IsComputed == nil || !c.IsComputed.GetValue() {
		t.Errorf("drifted: hydrated true must survive, got %v", c.IsComputed)
	}
}
