package conn_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/conntest"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
)

// TestEmulator_ColumnHydrationCoverage answers one question per column
// attribute in the Terraform model: can Read recover it from
// INFORMATION_SCHEMA alone? It creates one table exercising every modeled
// attribute, then runs the same queries the table hydration path uses and
// reports the raw values, including any normalization of user-written
// expressions (defaults, generation expressions).
func TestEmulator_ColumnHydrationCoverage(t *testing.T) {
	cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
	ctx := context.Background()

	fds, err := os.ReadFile("testdata/tftest.pb")
	if err != nil {
		t.Fatalf("read descriptor set (regenerate per testdata/tftest.proto): %v", err)
	}
	protoBundleOK := true
	if err := cn.ExecuteDDLWithDescriptors(ctx, db, fds, "CREATE PROTO BUNDLE (`tftest.Simple`)"); err != nil {
		protoBundleOK = false
		t.Logf("FINDING: emulator rejects CREATE PROTO BUNDLE: %v", err)
	}

	// One table exercising every attribute; a child table for interleave.
	if err := cn.ExecuteDDL(ctx, db,
		`CREATE TABLE probe (
			id INT64 NOT NULL,
			sub_id STRING(64) NOT NULL,
			str_max STRING(MAX),
			bytes_col BYTES(128),
			date_col DATE,
			json_col JSON,
			flt FLOAT64 NOT NULL DEFAULT (0.0),
			cnt INT64 DEFAULT (42),
			label STRING(50) DEFAULT ('hello'),
			flag BOOL DEFAULT (TRUE),
			created_at TIMESTAMP DEFAULT (CURRENT_TIMESTAMP()),
			update_time TIMESTAMP OPTIONS (allow_commit_timestamp=true),
			tags ARRAY<STRING(100)>,
			nums ARRAY<INT64>,
			scores ARRAY<FLOAT64>,
			gen_stored INT64 AS (cnt + 1) STORED
		) PRIMARY KEY (id, sub_id)`,
		`CREATE TABLE probe_child (
			id INT64 NOT NULL,
			sub_id STRING(64) NOT NULL,
			child_id INT64 NOT NULL
		) PRIMARY KEY (id, sub_id, child_id), INTERLEAVE IN PARENT probe ON DELETE CASCADE`,
	); err != nil {
		t.Fatalf("create probe tables: %v", err)
	}

	// Features the emulator may reject go in separate statements so one
	// rejection doesn't sink the whole probe.
	virtualGenOK := true
	if err := cn.ExecuteDDL(ctx, db, "ALTER TABLE probe ADD COLUMN gen_virtual INT64 AS (cnt + 2)"); err != nil {
		virtualGenOK = false
		t.Logf("FINDING: emulator rejects non-stored generated column: %v", err)
	}
	float32ArrayOK := true
	if err := cn.ExecuteDDL(ctx, db, "ALTER TABLE probe ADD COLUMN ratios ARRAY<FLOAT32>"); err != nil {
		float32ArrayOK = false
		t.Logf("FINDING: emulator rejects ARRAY<FLOAT32>: %v", err)
	}
	protoColOK := protoBundleOK
	if protoBundleOK {
		if err := cn.ExecuteDDL(ctx, db, "ALTER TABLE probe ADD COLUMN proto_col `tftest.Simple`"); err != nil {
			protoColOK = false
			t.Logf("FINDING: emulator rejects PROTO column: %v", err)
		}
	}

	// Same result shapes and queries as the table hydration path.
	type columnRow struct {
		ColumnName     sql.NullString `gorm:"column:COLUMN_NAME"`
		SpannerType    sql.NullString `gorm:"column:SPANNER_TYPE"`
		IsNullable     sql.NullString `gorm:"column:IS_NULLABLE"`
		ColumnDefault  sql.NullString `gorm:"column:COLUMN_DEFAULT"`
		IsGenerated    sql.NullString `gorm:"column:IS_GENERATED"`
		IsStored       sql.NullString `gorm:"column:IS_STORED"`
		GenerationExpr sql.NullString `gorm:"column:GENERATION_EXPRESSION"`
	}
	var rows []*columnRow
	if err := cn.Query(
		ctx,
		db,
		&rows,
		`SELECT COLUMN_NAME,SPANNER_TYPE,IS_NULLABLE,COLUMN_DEFAULT,IS_GENERATED,IS_STORED,GENERATION_EXPRESSION FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? ORDER BY ORDINAL_POSITION`,
		"probe",
	); err != nil {
		t.Fatalf("INFORMATION_SCHEMA.COLUMNS: %v", err)
	}
	byName := map[string]*columnRow{}
	for _, r := range rows {
		byName[r.ColumnName.String] = r
		t.Logf("COLUMNS %-12s type=%-22q nullable=%-4q default=%q generated=%q stored=%q expr=%q",
			r.ColumnName.String, r.SpannerType.String, r.IsNullable.String,
			r.ColumnDefault.String, r.IsGenerated.String, r.IsStored.String, r.GenerationExpr.String)
	}

	t.Run("type and size via SPANNER_TYPE", func(t *testing.T) {
		want := map[string]string{
			"id":         "INT64",
			"sub_id":     "STRING(64)",
			"str_max":    "STRING(MAX)",
			"bytes_col":  "BYTES(128)",
			"date_col":   "DATE",
			"json_col":   "JSON",
			"flt":        "FLOAT64",
			"flag":       "BOOL",
			"created_at": "TIMESTAMP",
			"tags":       "ARRAY<STRING(100)>",
			"nums":       "ARRAY<INT64>",
			"scores":     "ARRAY<FLOAT64>",
		}
		if float32ArrayOK {
			want["ratios"] = "ARRAY<FLOAT32>"
		}
		for col, typ := range want {
			r, ok := byName[col]
			if !ok {
				t.Errorf("%s: missing from INFORMATION_SCHEMA.COLUMNS", col)
				continue
			}
			if r.SpannerType.String != typ {
				t.Errorf("%s: SPANNER_TYPE = %q, want %q", col, r.SpannerType.String, typ)
			}
		}
	})

	t.Run("required via IS_NULLABLE", func(t *testing.T) {
		for col, want := range map[string]string{"id": "NO", "sub_id": "NO", "flt": "NO", "str_max": "YES", "cnt": "YES"} {
			if got := byName[col].IsNullable.String; got != want {
				t.Errorf("%s: IS_NULLABLE = %q, want %q", col, got, want)
			}
		}
	})

	t.Run("default_value via COLUMN_DEFAULT", func(t *testing.T) {
		// The interesting part is normalization: does Spanner return the
		// user's expression text byte-identical, or rewritten?
		written := map[string]string{
			"flt":        "0.0",
			"cnt":        "42",
			"label":      "'hello'",
			"flag":       "TRUE",
			"created_at": "CURRENT_TIMESTAMP()",
		}
		for col, wrote := range written {
			r := byName[col]
			if !r.ColumnDefault.Valid {
				t.Errorf("%s: COLUMN_DEFAULT is NULL, default not surfaced", col)
				continue
			}
			verdict := "IDENTICAL"
			if r.ColumnDefault.String != wrote {
				verdict = "NORMALIZED"
			}
			t.Logf("default %-10s wrote=%-22q got=%-22q %s", col, wrote, r.ColumnDefault.String, verdict)
		}
		if byName["str_max"].ColumnDefault.Valid {
			t.Errorf("str_max: COLUMN_DEFAULT = %q, want NULL for column without default", byName["str_max"].ColumnDefault.String)
		}
	})

	t.Run("generated columns via IS_GENERATED/IS_STORED/GENERATION_EXPRESSION", func(t *testing.T) {
		g := byName["gen_stored"]
		if g.IsGenerated.String != "ALWAYS" || g.IsStored.String != "YES" {
			t.Errorf("gen_stored: IS_GENERATED = %q, IS_STORED = %q; want ALWAYS, YES", g.IsGenerated.String, g.IsStored.String)
		}
		verdict := "IDENTICAL"
		if g.GenerationExpr.String != "cnt + 1" {
			verdict = "NORMALIZED"
		}
		t.Logf("generation expr wrote=%q got=%q %s", "cnt + 1", g.GenerationExpr.String, verdict)

		if c := byName["cnt"]; c.IsGenerated.String == "ALWAYS" {
			t.Errorf("cnt: IS_GENERATED = ALWAYS for a plain column")
		}
		if virtualGenOK {
			v := byName["gen_virtual"]
			if v.IsGenerated.String != "ALWAYS" || v.IsStored.String != "NO" {
				t.Errorf("gen_virtual: IS_GENERATED = %q, IS_STORED = %q; want ALWAYS, NO", v.IsGenerated.String, v.IsStored.String)
			}
		}
	})

	t.Run("primary key membership and order via INDEX_COLUMNS", func(t *testing.T) {
		type pkRow struct {
			ColumnName sql.NullString `gorm:"column:COLUMN_NAME"`
		}
		var pks []*pkRow
		if err := cn.Query(
			ctx,
			db,
			&pks,
			`SELECT COLUMN_NAME, ORDINAL_POSITION FROM INFORMATION_SCHEMA.INDEX_COLUMNS WHERE TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY_KEY' ORDER BY ORDINAL_POSITION`,
			"probe",
		); err != nil {
			t.Fatalf("INDEX_COLUMNS: %v", err)
		}
		var got []string
		for _, r := range pks {
			got = append(got, r.ColumnName.String)
		}
		if strings.Join(got, ",") != "id,sub_id" {
			t.Errorf("primary key = %v, want [id sub_id] in declaration order", got)
		}
	})

	t.Run("auto_update_time via COLUMN_OPTIONS", func(t *testing.T) {
		type optRow struct {
			ColumnName  string `gorm:"column:COLUMN_NAME"`
			OptionName  string `gorm:"column:OPTION_NAME"`
			OptionValue string `gorm:"column:OPTION_VALUE"`
		}
		var opts []optRow
		if err := cn.Query(
			ctx,
			db,
			&opts,
			"SELECT COLUMN_NAME, OPTION_NAME, OPTION_VALUE FROM INFORMATION_SCHEMA.COLUMN_OPTIONS WHERE TABLE_NAME = ?",
			"probe",
		); err != nil {
			t.Fatalf("COLUMN_OPTIONS: %v", err)
		}
		found := false
		for _, o := range opts {
			t.Logf("COLUMN_OPTIONS %s %s=%s", o.ColumnName, o.OptionName, o.OptionValue)
			if o.ColumnName == "update_time" && o.OptionName == "allow_commit_timestamp" && o.OptionValue == "TRUE" {
				found = true
			}
		}
		if !found {
			t.Error("allow_commit_timestamp=TRUE not surfaced for update_time")
		}
	})

	t.Run("proto_package via SPANNER_TYPE of proto column", func(t *testing.T) {
		if !protoColOK {
			t.Skip("proto column not created")
		}
		// Re-query: proto_col was added after the first snapshot.
		var pr []*columnRow
		if err := cn.Query(ctx, db, &pr,
			`SELECT COLUMN_NAME,SPANNER_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?`,
			"probe", "proto_col"); err != nil || len(pr) != 1 {
			t.Fatalf("proto_col row: %v (rows=%d)", err, len(pr))
		}
		got := pr[0].SpannerType.String
		t.Logf("proto_col SPANNER_TYPE = %q", got)
		// The fallback parser extracts the package from a PROTO<pkg> shape;
		// anything else means proto_package is NOT recoverable as-is.
		if !strings.HasPrefix(got, "PROTO<") {
			t.Logf("FINDING: SPANNER_TYPE is not in PROTO<pkg> shape; parseSpannerProtoPackage would return empty for %q", got)
		}
	})

	t.Run("interleave via TABLES", func(t *testing.T) {
		type tableRow struct {
			TableName       sql.NullString `gorm:"column:TABLE_NAME"`
			ParentTableName sql.NullString `gorm:"column:PARENT_TABLE_NAME"`
			OnDeleteAction  sql.NullString `gorm:"column:ON_DELETE_ACTION"`
			InterleaveType  sql.NullString `gorm:"column:INTERLEAVE_TYPE"`
		}
		var row tableRow
		if err := cn.Query(ctx, db, &row,
			`SELECT TABLE_NAME,PARENT_TABLE_NAME,ON_DELETE_ACTION,INTERLEAVE_TYPE FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ?`,
			"probe_child"); err != nil {
			t.Fatalf("TABLES row: %v", err)
		}
		t.Logf("probe_child parent=%q on_delete=%q interleave_type=%q",
			row.ParentTableName.String, row.OnDeleteAction.String, row.InterleaveType.String)
		if row.ParentTableName.String != "probe" || row.OnDeleteAction.String != "CASCADE" || row.InterleaveType.String != "IN PARENT" {
			t.Errorf("interleave = (%q, %q, %q), want (probe, CASCADE, IN PARENT)",
				row.ParentTableName.String, row.OnDeleteAction.String, row.InterleaveType.String)
		}
	})
}
