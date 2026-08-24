package spanner

import (
	"testing"

	tableschema "terraform-provider-alis/internal/spanner/schema"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Interleave is Optional and replace-only, so a refresh must never hydrate
// values the configuration did not express: introducing an interleave block
// the practitioner omitted turns an adopted interleaved table into a planned
// table replace, and INFORMATION_SCHEMA always answers on_delete (NO ACTION)
// even when the configuration left it unset.
func TestPreserveUnsetInterleave(t *testing.T) {
	dbInterleave := func(onDelete tableschema.SpannerTableConstraintAction) *tableschema.SpannerTableInterleave {
		return &tableschema.SpannerTableInterleave{ParentTable: "parent", OnDelete: onDelete}
	}

	tests := []struct {
		name  string
		prior *spannerTableInterleave
		db    *tableschema.SpannerTableInterleave
		want  *spannerTableInterleave
	}{
		{
			name:  "never introduced when prior state omitted it",
			prior: nil,
			db:    dbInterleave(tableschema.SpannerTableConstraintNoAction),
			want:  nil,
		},
		{
			name:  "nil when neither side has it",
			prior: nil,
			db:    nil,
			want:  nil,
		},
		{
			name:  "cleared when the database is no longer interleaved",
			prior: &spannerTableInterleave{ParentTable: types.StringValue("parent")},
			db:    nil,
			want:  nil,
		},
		{
			name:  "on_delete stays unset when prior state left it unset",
			prior: &spannerTableInterleave{ParentTable: types.StringValue("parent")},
			db:    dbInterleave(tableschema.SpannerTableConstraintNoAction),
			want:  &spannerTableInterleave{ParentTable: types.StringValue("parent"), OnDelete: types.StringNull()},
		},
		{
			name: "on_delete hydrates from the database when prior state set it",
			prior: &spannerTableInterleave{
				ParentTable: types.StringValue("parent"),
				OnDelete:    types.StringValue("NO ACTION"),
			},
			db: dbInterleave(tableschema.SpannerTableConstraintActionCascade),
			want: &spannerTableInterleave{
				ParentTable: types.StringValue("parent"),
				OnDelete:    types.StringValue("CASCADE"),
			},
		},
		{
			name: "parent_table always reflects the database",
			prior: &spannerTableInterleave{
				ParentTable: types.StringValue("old_parent"),
			},
			db:   &tableschema.SpannerTableInterleave{ParentTable: "new_parent"},
			want: &spannerTableInterleave{ParentTable: types.StringValue("new_parent"), OnDelete: types.StringNull()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := preserveUnsetInterleave(tc.prior, tc.db)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if got == nil {
				return
			}
			if !got.ParentTable.Equal(tc.want.ParentTable) {
				t.Errorf("ParentTable = %v, want %v", got.ParentTable, tc.want.ParentTable)
			}
			if !got.OnDelete.Equal(tc.want.OnDelete) {
				t.Errorf("OnDelete = %v, want %v", got.OnDelete, tc.want.OnDelete)
			}
		})
	}
}
