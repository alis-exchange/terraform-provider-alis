package schema

import "fmt"

// SpannerTableInterleave represents a Spanner table interleave.
type SpannerTableInterleave struct {
	// The name of the parent table.
	ParentTable string
	// Referential actions on delete
	OnDelete SpannerTableConstraintAction
}

// GetParentTable returns the parent table name.
func (i *SpannerTableInterleave) GetParentTable() string {
	if i == nil {
		return ""
	}

	return i.ParentTable
}

// GetOnDelete returns the referential action on delete.
func (i *SpannerTableInterleave) GetOnDelete() SpannerTableConstraintAction {
	if i == nil {
		return SpannerTableConstraintActionUnspecified
	}

	return i.OnDelete
}

// ddl renders the interleave clause. An ON DELETE action of CASCADE selects
// the INTERLEAVE IN PARENT form with its ON DELETE clause; unspecified or
// NO ACTION renders the plain INTERLEAVE IN form.
func (i *SpannerTableInterleave) ddl() string {
	if i == nil {
		return ""
	}

	// Add interleave
	if i.GetOnDelete() == SpannerTableConstraintActionUnspecified || i.GetOnDelete() == SpannerTableConstraintNoAction {
		return "INTERLEAVE IN " + i.GetParentTable()
	}

	return fmt.Sprintf("INTERLEAVE IN PARENT %s ON DELETE %s", i.GetParentTable(), i.GetOnDelete().String())
}
