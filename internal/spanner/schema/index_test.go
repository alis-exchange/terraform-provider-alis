package schema

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestSpannerTableIndexCreateDdl(t *testing.T) {
	tests := []struct {
		name    string
		index   *SpannerTableIndex
		table   string
		want    string
		wantErr bool
	}{
		{
			// The double space after CREATE matches the statement Spanner has
			// been accepting from this provider all along; kept byte-identical.
			name: "non-unique single column defaults to ASC",
			index: &SpannerTableIndex{
				Name:    "display_name_idx",
				Columns: []*SpannerTableIndexColumn{{Name: "display_name"}},
			},
			table: "tf_test",
			want:  "CREATE  INDEX display_name_idx ON tf_test (display_name ASC)",
		},
		{
			name: "unique multi-column with explicit orders",
			index: &SpannerTableIndex{
				Name: "by_name_date",
				Columns: []*SpannerTableIndexColumn{
					{Name: "display_name", Order: SpannerTableIndexColumnOrder_ASC},
					{Name: "inception_date", Order: SpannerTableIndexColumnOrder_DESC},
				},
				Unique: wrapperspb.Bool(true),
			},
			table: "tf_test",
			want:  "CREATE UNIQUE INDEX by_name_date ON tf_test (display_name ASC, inception_date DESC)",
		},
		{
			name: "unique false renders like nil",
			index: &SpannerTableIndex{
				Name:    "idx",
				Columns: []*SpannerTableIndexColumn{{Name: "c"}},
				Unique:  wrapperspb.Bool(false),
			},
			table: "t",
			want:  "CREATE  INDEX idx ON t (c ASC)",
		},
		{
			name:    "missing table errors",
			index:   &SpannerTableIndex{Name: "idx", Columns: []*SpannerTableIndexColumn{{Name: "c"}}},
			table:   "",
			wantErr: true,
		},
		{
			name:    "missing columns errors",
			index:   &SpannerTableIndex{Name: "idx"},
			table:   "t",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.index.CreateDdl(tc.table)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CreateDdl() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("CreateDdl() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("nil receiver returns empty", func(t *testing.T) {
		var idx *SpannerTableIndex
		got, err := idx.CreateDdl("t")
		if got != "" || err != nil {
			t.Errorf("nil receiver = (%q, %v), want empty", got, err)
		}
	})

	t.Run("UNSPECIFIED order renders ASC without mutating the input", func(t *testing.T) {
		col := &SpannerTableIndexColumn{Name: "c"}
		idx := &SpannerTableIndex{Name: "idx", Columns: []*SpannerTableIndexColumn{col}}
		if _, err := idx.CreateDdl("t"); err != nil {
			t.Fatal(err)
		}
		if col.Order != SpannerTableIndexColumnOrder_UNSPECIFIED {
			t.Errorf("builder mutated input column order to %v", col.Order)
		}
	})
}

func TestDropIndexDdl(t *testing.T) {
	if got := DropIndexDdl("display_name_idx"); got != "DROP INDEX display_name_idx" {
		t.Errorf("DropIndexDdl() = %q", got)
	}
}
