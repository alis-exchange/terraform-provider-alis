package schema

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// computedColumn returns a valid computed column; tests vary IsStored on copies of it.
func computedColumn() *SpannerTableColumn {
	return &SpannerTableColumn{
		Name:           "full_name",
		Type:           "STRING(MAX)",
		IsComputed:     wrapperspb.Bool(true),
		ComputationDdl: wrapperspb.String("CONCAT(first_name, ' ', last_name)"),
	}
}

// pkColumn returns a valid primary-key column for classification tests.
func pkColumn() *SpannerTableColumn {
	return &SpannerTableColumn{
		Name:         "user_id",
		Type:         "STRING(36)",
		IsPrimaryKey: wrapperspb.Bool(true),
	}
}

// plainColumn returns a valid non-key, non-computed column.
func plainColumn() *SpannerTableColumn {
	return &SpannerTableColumn{
		Name:     "email",
		Type:     "STRING(MAX)",
		Required: wrapperspb.Bool(true),
	}
}

func TestClassifyColumnChange(t *testing.T) {
	withIsStored := func(c *SpannerTableColumn, v *wrapperspb.BoolValue) *SpannerTableColumn {
		c.IsStored = v
		return c
	}
	withPK := func(c *SpannerTableColumn, v *wrapperspb.BoolValue) *SpannerTableColumn {
		c.IsPrimaryKey = v
		return c
	}
	withType := func(c *SpannerTableColumn, ty string) *SpannerTableColumn {
		c.Type = ty
		return c
	}
	withComputationDdl := func(c *SpannerTableColumn, ddl string) *SpannerTableColumn {
		c.ComputationDdl = wrapperspb.String(ddl)
		return c
	}
	withoutComputed := func(c *SpannerTableColumn) *SpannerTableColumn {
		c.IsComputed = nil
		c.ComputationDdl = nil
		return c
	}
	withSize := func(c *SpannerTableColumn, size int64) *SpannerTableColumn {
		c.Size = wrapperspb.Int64(size)
		return c
	}
	withRequired := func(c *SpannerTableColumn, v *wrapperspb.BoolValue) *SpannerTableColumn {
		c.Required = v
		return c
	}
	withDefault := func(c *SpannerTableColumn, v string) *SpannerTableColumn {
		c.DefaultValue = wrapperspb.String(v)
		return c
	}

	tests := []struct {
		name       string
		prior      *SpannerTableColumn
		planned    *SpannerTableColumn
		wantClass  ColumnChangeClass
		wantReason string
	}{
		{
			"identical",
			plainColumn(),
			plainColumn(),
			ColumnUnchanged,
			"",
		},
		{
			"both nil",
			nil,
			nil,
			ColumnUnchanged,
			"",
		},

		// New / removed columns
		{
			"new non-PK column is alterable",
			nil,
			plainColumn(),
			ColumnAlterable,
			"",
		},
		{
			"new PK column requires replace",
			nil, pkColumn(),
			ColumnRequiresReplace,
			`Column "user_id" is a new primary key column and requires a table replace`,
		},
		{
			"removed non-PK column is alterable",
			plainColumn(),
			nil,
			ColumnAlterable,
			"",
		},
		{
			"removed PK column requires replace",
			pkColumn(),
			nil,
			ColumnRequiresReplace,
			`Column "user_id" is a removed primary key column and requires a table replace`,
		},

		// Type
		{
			"type change requires replace",
			plainColumn(),
			withType(plainColumn(), "INT64"),
			ColumnRequiresReplace,
			`Column "email" has a changed type and requires a table replace`,
		},

		// Primary key status (null ≡ false)
		{
			"PK false to true requires replace",
			withPK(plainColumn(), wrapperspb.Bool(false)),
			withPK(plainColumn(), wrapperspb.Bool(true)),
			ColumnRequiresReplace,
			`Column "email" has a changed primary key status and requires a table replace`,
		},
		{
			"PK true to nil requires replace",
			withPK(plainColumn(), wrapperspb.Bool(true)),
			withPK(plainColumn(), nil),
			ColumnRequiresReplace,
			`Column "email" has a changed primary key status and requires a table replace`,
		},
		{
			"PK nil to false is unchanged",
			withPK(plainColumn(), nil),
			withPK(plainColumn(), wrapperspb.Bool(false)),
			ColumnUnchanged,
			"",
		},

		// Computed columns
		{
			"computation_ddl change while computed requires replace",
			computedColumn(),
			withComputationDdl(computedColumn(), "LOWER(email)"),
			ColumnRequiresReplace,
			`Column "full_name" has a changed computation_ddl or is_computed has been disabled and requires a table replace`,
		},
		{
			"is_computed disabled requires replace",
			computedColumn(),
			withoutComputed(computedColumn()),
			ColumnRequiresReplace,
			`Column "full_name" has a changed computation_ddl or is_computed has been disabled and requires a table replace`,
		},
		{
			"is_computed enabled is alterable (today's behavior)",
			withoutComputed(computedColumn()),
			computedColumn(),
			ColumnAlterable,
			"",
		},

		// is_stored: an unset PLANNED value means "no opinion" — is_stored is
		// Computed, so an omitted config value plans as unknown and inherits
		// the prior state via UseStateForUnknown. Forcing a replace here would
		// destroy every v1.x table with a computed column (v1 always created
		// them STORED and had no is_stored attribute to say so). Alterable is
		// acceptable: the resolved plan equals prior state, so no update runs.
		{
			"is_stored true to nil planned is not a replace",
			withIsStored(computedColumn(), wrapperspb.Bool(true)),
			withIsStored(computedColumn(), nil),
			ColumnAlterable,
			"",
		},
		{
			"is_stored true to explicit false requires replace",
			withIsStored(computedColumn(), wrapperspb.Bool(true)),
			withIsStored(computedColumn(), wrapperspb.Bool(false)),
			ColumnRequiresReplace,
			`Column "full_name" has a changed is_stored status and requires a table replace`,
		},
		{
			"is_stored nil to true requires replace",
			withIsStored(computedColumn(), nil),
			withIsStored(computedColumn(), wrapperspb.Bool(true)),
			ColumnRequiresReplace,
			`Column "full_name" has a changed is_stored status and requires a table replace`,
		},
		{
			"is_stored nil to false is unchanged",
			withIsStored(computedColumn(), nil),
			withIsStored(computedColumn(), wrapperspb.Bool(false)),
			ColumnUnchanged,
			"",
		},

		// In-place alterations
		{
			"size change is alterable",
			withSize(plainColumn(), 100),
			withSize(plainColumn(), 200),
			ColumnAlterable,
			"",
		},
		{
			"required change is alterable",
			withRequired(plainColumn(), wrapperspb.Bool(true)),
			withRequired(plainColumn(), wrapperspb.Bool(false)),
			ColumnAlterable,
			"",
		},
		{
			"default change is alterable",
			plainColumn(),
			withDefault(plainColumn(), "'none'"),
			ColumnAlterable,
			"",
		},

		// Multiple rules firing: class wins, reason is the first rule in closure order (type first)
		{
			"type and is_stored both changed requires replace",
			computedColumn(),
			withIsStored(withType(computedColumn(), "BYTES(MAX)"), wrapperspb.Bool(true)),
			ColumnRequiresReplace,
			`Column "full_name" has a changed type and requires a table replace`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotClass, gotReason := ClassifyColumnChange(tc.prior, tc.planned)
			if gotClass != tc.wantClass {
				t.Errorf("ClassifyColumnChange() class = %v, want %v", gotClass, tc.wantClass)
			}
			if gotReason != tc.wantReason {
				t.Errorf("ClassifyColumnChange() reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}

func TestSpannerTableColumnCompare_IsStored(t *testing.T) {
	tests := []struct {
		name      string
		prior     *wrapperspb.BoolValue
		planned   *wrapperspb.BoolValue
		wantEqual bool
	}{
		{"true vs nil differs", wrapperspb.Bool(true), nil, false},
		{"nil vs true differs", nil, wrapperspb.Bool(true), false},
		{"true vs false differs", wrapperspb.Bool(true), wrapperspb.Bool(false), false},
		{"nil vs false equal (null means false)", nil, wrapperspb.Bool(false), true},
		{"true vs true equal", wrapperspb.Bool(true), wrapperspb.Bool(true), true},
		{"nil vs nil equal", nil, nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prior, planned := computedColumn(), computedColumn()
			prior.IsStored = tc.prior
			planned.IsStored = tc.planned
			if got := prior.compare(planned); got != tc.wantEqual {
				t.Errorf("compare() = %v, want %v (IsStored prior=%v planned=%v)",
					got, tc.wantEqual, tc.prior.GetValue(), tc.planned.GetValue())
			}
		})
	}
}
