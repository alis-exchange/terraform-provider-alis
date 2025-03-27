package schema

type SpannerTableForeignKeyConstraint struct {
	// The name of the constraint
	Name string
	// Referenced table
	ReferencedTable string
	// Referenced column
	ReferencedColumn string
	// Referencing column
	Column string
	// Referential actions on delete
	OnDelete SpannerTableConstraintAction
}

type SpannerTableConstraintAction int64

const (
	SpannerTableConstraintActionUnspecified SpannerTableConstraintAction = iota
	SpannerTableConstraintActionCascade
	SpannerTableConstraintNoAction
)

func (a SpannerTableConstraintAction) String() string {
	return [...]string{"", "CASCADE", "NO ACTION"}[a]
}

func SpannerTableConstraintActionFromString(s string) SpannerTableConstraintAction {
	switch s {
	case "CASCADE":
		return SpannerTableConstraintActionCascade
	case "NO ACTION":
		return SpannerTableConstraintNoAction
	default:
		return SpannerTableConstraintActionUnspecified
	}
}

var SpannerTableConstraintActions = []string{
	SpannerTableConstraintActionCascade.String(),
	SpannerTableConstraintNoAction.String(),
}
