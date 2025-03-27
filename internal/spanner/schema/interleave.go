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

func (i *SpannerTableInterleave) ddl() (string, error) {
	if i == nil {
		return "", nil
	}

	// Add interleave
	if i.GetOnDelete() == SpannerTableConstraintActionUnspecified || i.GetOnDelete() == SpannerTableConstraintNoAction {
		return fmt.Sprintf("INTERLEAVE IN %s", i.GetParentTable()), nil
	}

	return fmt.Sprintf("INTERLEAVE IN PARENT %s ON DELETE %s", i.GetParentTable(), i.GetOnDelete().String()), nil
}
