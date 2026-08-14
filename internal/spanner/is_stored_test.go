package spanner

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// is_stored must be Computed so an omitted config value plans as unknown and
// UseStateForUnknown can inherit the database's actual storedness. Without
// this, every v1.x upgrader with a computed column (always STORED, with no
// way to say so in config) gets a forced table replace.
func TestSpannerTableSchema_IsStoredComputed(t *testing.T) {
	resp := &resource.SchemaResponse{}
	NewSpannerTableResource().Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	isStored := resp.Schema.Attributes["schema"].(rschema.SingleNestedAttribute).
		Attributes["columns"].(rschema.ListNestedAttribute).
		NestedObject.Attributes["is_stored"].(rschema.BoolAttribute)

	if !isStored.Optional || !isStored.Computed {
		t.Errorf("is_stored Optional=%t Computed=%t, want both true", isStored.Optional, isStored.Computed)
	}
	if len(isStored.PlanModifiers) == 0 {
		t.Error("is_stored has no plan modifiers, want UseStateForUnknown")
	}
}

func testColumnList(t *testing.T, cols []spannerTableColumn) types.List {
	t.Helper()
	ctx := context.Background()
	objType := types.ObjectType{AttrTypes: spannerTableColumn{}.attrTypes()}
	values := make([]attr.Value, 0, len(cols))
	for _, c := range cols {
		obj, d := types.ObjectValueFrom(ctx, objType.AttrTypes, c)
		if d.HasError() {
			t.Fatalf("building column object: %v", d)
		}
		values = append(values, obj)
	}
	list, d := types.ListValue(objType, values)
	if d.HasError() {
		t.Fatalf("building column list: %v", d)
	}
	return list
}

// During plan modification a Computed is_stored is still unknown; the
// wrapper conversion must treat that as unset, not as an explicit false —
// a set-false wrapper against a stored prior column reads as a change and
// forces a replace.
func TestTableColumnsToSchema_UnknownIsStoredIsUnset(t *testing.T) {
	list := testColumnList(t, []spannerTableColumn{{
		Name:           types.StringValue("full_name"),
		Type:           types.StringValue("STRING"),
		IsComputed:     types.BoolValue(true),
		ComputationDdl: types.StringValue("CONCAT(first_name, last_name)"),
		IsStored:       types.BoolUnknown(),
	}})

	cols, d := tableColumnsToSchema(context.Background(), list)
	if d.HasError() {
		t.Fatalf("tableColumnsToSchema: %v", d)
	}
	if got := cols[0].GetIsStored(); got != nil {
		t.Errorf("unknown is_stored converted to wrapper %v, want nil (unset)", got)
	}
}

// Create and Update persist the plan as state, and a brand-new computed
// column with omitted is_stored reaches them still unknown (no prior state
// for UseStateForUnknown to inherit). Unknown values cannot be stored, so
// they resolve to null — matching how Read collapses hydrated booleans the
// config never set.
func TestResolveUnknownIsStored(t *testing.T) {
	ctx := context.Background()
	list := testColumnList(t, []spannerTableColumn{
		{Name: types.StringValue("a"), Type: types.StringValue("STRING"), IsStored: types.BoolUnknown()},
		{Name: types.StringValue("b"), Type: types.StringValue("STRING"), IsStored: types.BoolValue(true)},
		{Name: types.StringValue("c"), Type: types.StringValue("STRING")},
	})

	resolved, d := resolveUnknownIsStored(ctx, list)
	if d.HasError() {
		t.Fatalf("resolveUnknownIsStored: %v", d)
	}

	var cols []spannerTableColumn
	if d := resolved.ElementsAs(ctx, &cols, false); d.HasError() {
		t.Fatalf("decoding resolved columns: %v", d)
	}
	if !cols[0].IsStored.IsNull() {
		t.Errorf("unknown is_stored resolved to %v, want null", cols[0].IsStored)
	}
	if !cols[1].IsStored.ValueBool() {
		t.Error("explicit is_stored=true must survive resolution")
	}
	if !cols[2].IsStored.IsNull() {
		t.Error("null is_stored must stay null")
	}
}
