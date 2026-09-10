package spanner

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// runRequireReplaceWithState drives the handler with a full prior state, as
// the framework does, so the destroy protection can read it.
func runRequireReplaceWithState(
	t *testing.T,
	state tfsdk.State,
	prior, planned []spannerTableColumn,
) *listplanmodifier.RequiresReplaceIfFuncResponse {
	t.Helper()
	resp := &listplanmodifier.RequiresReplaceIfFuncResponse{}
	tableColumnsRequireReplace(context.Background(), planmodifier.ListRequest{
		Path:       path.Root("schema").AtName("columns"),
		State:      state,
		StateValue: columnList(t, prior),
		PlanValue:  columnList(t, planned),
	}, resp)
	return resp
}

// runRequireReplace classifies columns without a prior state to consult, which
// must never error: a state carrying no schema has no protection to read.
func runRequireReplace(t *testing.T, prior, planned []spannerTableColumn) *listplanmodifier.RequiresReplaceIfFuncResponse {
	t.Helper()
	resp := runRequireReplaceWithState(t, tfsdk.State{}, prior, planned)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error without a prior state: %v", resp.Diagnostics.Errors())
	}
	return resp
}

func TestTableColumnsRequireReplace(t *testing.T) {
	storedTrue := fullColumnModel()
	storedNull := fullColumnModel()
	storedNull.IsStored = types.BoolNull()
	storedFalse := fullColumnModel()
	storedFalse.IsStored = types.BoolValue(false)

	typeChanged := minimalColumnModel()
	typeChanged.Type = types.StringValue("INT64")

	sizeChanged := minimalColumnModel()
	sizeChanged.Size = types.Int64Value(64)

	newPK := spannerTableColumn{
		Name:         types.StringValue("new_key"),
		Type:         types.StringValue("INT64"),
		IsPrimaryKey: types.BoolValue(true),
	}

	t.Run("identical columns do not replace", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{minimalColumnModel()}, []spannerTableColumn{minimalColumnModel()})
		if resp.RequiresReplace || resp.Diagnostics.WarningsCount() != 0 {
			t.Errorf("RequiresReplace=%v warnings=%d, want false/0", resp.RequiresReplace, resp.Diagnostics.WarningsCount())
		}
	})

	// An unset planned is_stored inherits the prior value (the attribute is
	// Computed with UseStateForUnknown), so only an explicit flip replaces.
	// v1.x state always carries is_stored=true for computed columns while
	// v1.x configs cannot mention it; treating unset as false would force a
	// table replace on every upgraded config.
	t.Run("is_stored unset planned does not replace", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{storedTrue}, []spannerTableColumn{storedNull})
		if resp.RequiresReplace || resp.Diagnostics.WarningsCount() != 0 {
			t.Errorf("RequiresReplace=%v warnings=%d, want false/0", resp.RequiresReplace, resp.Diagnostics.WarningsCount())
		}
	})

	t.Run("is_stored explicit flip replaces with warning", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{storedTrue}, []spannerTableColumn{storedFalse})
		if !resp.RequiresReplace {
			t.Fatal("RequiresReplace = false, want true")
		}
		warns := resp.Diagnostics.Warnings()
		if len(warns) != 1 ||
			warns[0].Summary() != `Column "proto_col" requires a table replace` ||
			warns[0].Detail() != `Column "proto_col" has a changed is_stored status and requires a table replace` {
			t.Errorf("unexpected warnings: %v", warns)
		}
	})

	t.Run("type change replaces", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{minimalColumnModel()}, []spannerTableColumn{typeChanged})
		if !resp.RequiresReplace ||
			resp.Diagnostics.Warnings()[0].Detail() != `Column "email" has a changed type and requires a table replace` {
			t.Errorf("RequiresReplace=%v warnings=%v", resp.RequiresReplace, resp.Diagnostics.Warnings())
		}
	})

	t.Run("added primary key replaces", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{minimalColumnModel()}, []spannerTableColumn{minimalColumnModel(), newPK})
		if !resp.RequiresReplace {
			t.Fatal("RequiresReplace = false, want true")
		}
		if d := resp.Diagnostics.Warnings()[0].Detail(); d != `Column "new_key" is a new primary key column and requires a table replace` {
			t.Errorf("detail = %q", d)
		}
	})

	t.Run("added and removed non-key columns do not replace", func(t *testing.T) {
		resp := runRequireReplace(t,
			[]spannerTableColumn{minimalColumnModel(), {Name: types.StringValue("old"), Type: types.StringValue("INT64")}},
			[]spannerTableColumn{minimalColumnModel(), {Name: types.StringValue("fresh"), Type: types.StringValue("INT64")}})
		if resp.RequiresReplace || resp.Diagnostics.WarningsCount() != 0 {
			t.Errorf("RequiresReplace=%v warnings=%d, want false/0", resp.RequiresReplace, resp.Diagnostics.WarningsCount())
		}
	})

	t.Run("alterable size change does not replace", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{minimalColumnModel()}, []spannerTableColumn{sizeChanged})
		if resp.RequiresReplace || resp.Diagnostics.WarningsCount() != 0 {
			t.Errorf("RequiresReplace=%v warnings=%d, want false/0", resp.RequiresReplace, resp.Diagnostics.WarningsCount())
		}
	})
}

// A column change that cannot be applied in place replaces the table, so it is
// refused while the prior state protects the table from deletion.
func TestTableColumnsRequireReplacePreventDestroy(t *testing.T) {
	prior := []spannerTableColumn{minimalColumnModel()}

	typeChanged := minimalColumnModel()
	typeChanged.Type = types.StringValue("INT64")

	sizeChanged := minimalColumnModel()
	sizeChanged.Size = types.Int64Value(64)

	t.Run("protected state refuses a replacing change", func(t *testing.T) {
		resp := runRequireReplaceWithState(t, tableState(t, protectedTableJSON), prior, []spannerTableColumn{typeChanged})
		if !resp.RequiresReplace {
			t.Error("RequiresReplace = false, want true")
		}
		assertProtectionError(t, resp.Diagnostics, "Table Protected From Replacement", path.Root("schema").AtName("columns"))
		// The warning naming the column survives alongside the refusal.
		if warns := resp.Diagnostics.Warnings(); len(warns) != 1 || !strings.Contains(warns[0].Summary(), "email") {
			t.Errorf("warnings = %v, want one naming the column", warns)
		}
	})

	t.Run("unprotected state replaces with a warning", func(t *testing.T) {
		resp := runRequireReplaceWithState(t, tableState(t, unprotectedTableJSON), prior, []spannerTableColumn{typeChanged})
		if !resp.RequiresReplace || resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
			t.Errorf("RequiresReplace=%v diagnostics=%v", resp.RequiresReplace, resp.Diagnostics)
		}
	})

	t.Run("protected state allows an in-place change", func(t *testing.T) {
		resp := runRequireReplaceWithState(t, tableState(t, protectedTableJSON), prior, []spannerTableColumn{sizeChanged})
		if resp.RequiresReplace || resp.Diagnostics.HasError() {
			t.Errorf("RequiresReplace=%v diagnostics=%v, want an in-place update", resp.RequiresReplace, resp.Diagnostics)
		}
	})
}
