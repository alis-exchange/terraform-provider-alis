package spanner

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	tableschema "terraform-provider-alis/internal/spanner/schema"
)

// tableColumnsRequireReplace is the RequiresReplaceIf handler for schema.columns.
// It pairs prior and planned columns by name and forces a table replace whenever
// schema.ClassifyColumnChange reports a change that cannot be applied in place,
// emitting one warning per affected column.
func tableColumnsRequireReplace(ctx context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
	priorColumns, d := tableColumnsToSchema(ctx, req.StateValue)
	resp.Diagnostics.Append(d...)
	if d.HasError() {
		return
	}
	plannedColumns, d := tableColumnsToSchema(ctx, req.PlanValue)
	resp.Diagnostics.Append(d...)
	if d.HasError() {
		return
	}

	type columnPair struct {
		prior   *tableschema.SpannerTableColumn
		planned *tableschema.SpannerTableColumn
	}
	pairs := make(map[string]*columnPair)
	for _, column := range priorColumns {
		pairs[column.GetName()] = &columnPair{prior: column}
	}
	for _, column := range plannedColumns {
		if pair, ok := pairs[column.GetName()]; ok {
			pair.planned = column
		} else {
			pairs[column.GetName()] = &columnPair{planned: column}
		}
	}

	// Deterministic warning order (the closure iterated the map unordered).
	names := make([]string, 0, len(pairs))
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		pair := pairs[name]
		class, reason := tableschema.ClassifyColumnChange(pair.prior, pair.planned)
		if class == tableschema.ColumnRequiresReplace {
			resp.RequiresReplace = true
			resp.Diagnostics.AddWarning(fmt.Sprintf("Column %q requires a table replace", name), reason)
		}
	}
}
