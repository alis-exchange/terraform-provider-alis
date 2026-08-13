package spanner

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func runRequireReplace(t *testing.T, prior, planned []spannerTableColumn) *listplanmodifier.RequiresReplaceIfFuncResponse {
	t.Helper()
	resp := &listplanmodifier.RequiresReplaceIfFuncResponse{}
	tableColumnsRequireReplace(context.Background(), planmodifier.ListRequest{
		StateValue: columnList(t, prior),
		PlanValue:  columnList(t, planned),
	}, resp)
	return resp
}

func TestTableColumnsRequireReplace(t *testing.T) {
	storedTrue := fullColumnModel()
	storedNull := fullColumnModel()
	storedNull.IsStored = types.BoolNull()

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

	t.Run("is_stored flip replaces with warning", func(t *testing.T) {
		resp := runRequireReplace(t, []spannerTableColumn{storedTrue}, []spannerTableColumn{storedNull})
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
