package schema

import "fmt"

// ColumnChangeClass classifies how a planned column differs from its prior state.
type ColumnChangeClass int

const (
	// ColumnUnchanged means the two columns are identical.
	ColumnUnchanged ColumnChangeClass = iota
	// ColumnAlterable means the change can be applied without recreating the table.
	ColumnAlterable
	// ColumnRequiresReplace means the change requires dropping and recreating the table.
	ColumnRequiresReplace
)

// ClassifyColumnChange reports how a planned column differs from its prior state.
// prior == nil means the column is new; planned == nil means it is being removed.
// For replace decisions the returned reason is the user-facing sentence shown as a
// Terraform warning; it is empty otherwise. The rules and their order preserve the
// table resource's historical replace-on-change behavior exactly (null ≡ false for
// all boolean attributes).
func ClassifyColumnChange(prior, planned *SpannerTableColumn) (ColumnChangeClass, string) {
	if prior == nil && planned == nil {
		return ColumnUnchanged, ""
	}

	// New column: only a new primary-key column forces a replace.
	if prior == nil {
		if planned.GetIsPrimaryKey().GetValue() {
			return ColumnRequiresReplace, fmt.Sprintf("Column %q is a new primary key column and requires a table replace", planned.GetName())
		}
		return ColumnAlterable, ""
	}

	// Removed column: only a removed primary-key column forces a replace.
	if planned == nil {
		if prior.GetIsPrimaryKey().GetValue() {
			return ColumnRequiresReplace, fmt.Sprintf("Column %q is a removed primary key column and requires a table replace", prior.GetName())
		}
		return ColumnAlterable, ""
	}

	name := planned.GetName()

	if prior.Type != planned.Type {
		return ColumnRequiresReplace, fmt.Sprintf("Column %q has a changed type and requires a table replace", name)
	}

	if prior.GetIsPrimaryKey().GetValue() != planned.GetIsPrimaryKey().GetValue() {
		return ColumnRequiresReplace, fmt.Sprintf("Column %q has a changed primary key status and requires a table replace", name)
	}

	// A changed computation on a computed column, or disabling is_computed, cannot
	// be altered in place. Enabling is_computed is deliberately not a replace —
	// that matches the historical plan-modifier behavior.
	priorComputed := prior.GetIsComputed().GetValue()
	plannedComputed := planned.GetIsComputed().GetValue()
	if (priorComputed && plannedComputed && prior.GetComputationDdl().GetValue() != planned.GetComputationDdl().GetValue()) ||
		(priorComputed && !plannedComputed) {
		return ColumnRequiresReplace, fmt.Sprintf("Column %q has a changed computation_ddl or is_computed has been disabled and requires a table replace", name)
	}

	if prior.GetIsStored().GetValue() != planned.GetIsStored().GetValue() {
		return ColumnRequiresReplace, fmt.Sprintf("Column %q has a changed is_stored status and requires a table replace", name)
	}

	if prior.compare(planned) {
		return ColumnUnchanged, ""
	}

	return ColumnAlterable, ""
}
