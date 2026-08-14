package spanner

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// unique and columns.order must be Computed with UseStateForUnknown so an
// omitted config value inherits the hydrated state instead of reading as a
// change. Both v1.x-written state and v2 refresh store the values explicitly
// ("asc", unique=false), so unset-means-different forces a pointless index
// replace on every upgraded or refreshed config.
func TestSpannerTableIndexSchema_InheritedAttributes(t *testing.T) {
	resp := &resource.SchemaResponse{}
	NewSpannerTableIndexResource().Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	unique := resp.Schema.Attributes["unique"].(rschema.BoolAttribute)
	if !unique.Optional || !unique.Computed {
		t.Errorf("unique Optional=%t Computed=%t, want both true", unique.Optional, unique.Computed)
	}
	// UseStateForUnknown must run before RequiresReplace so the replace check
	// compares the inherited value, not unknown.
	if len(unique.PlanModifiers) < 2 {
		t.Errorf("unique has %d plan modifiers, want UseStateForUnknown before RequiresReplace", len(unique.PlanModifiers))
	}

	order := resp.Schema.Attributes["columns"].(rschema.ListNestedAttribute).
		NestedObject.Attributes["order"].(rschema.StringAttribute)
	if !order.Optional || !order.Computed {
		t.Errorf("order Optional=%t Computed=%t, want both true", order.Optional, order.Computed)
	}
	if len(order.PlanModifiers) == 0 {
		t.Error("order has no plan modifiers, want UseStateForUnknown")
	}
}

// A brand-new index (or a newly added column) has no prior state for
// UseStateForUnknown to inherit, so unique and order reach Create still
// unknown — and unknown values cannot be persisted as state.
func TestResolveUnknownIndexInherited(t *testing.T) {
	ctx := context.Background()
	objType := types.ObjectType{AttrTypes: spannerTableIndexColumn{}.attrTypes()}
	columns, d := types.ListValueFrom(ctx, objType, []spannerTableIndexColumn{
		{Name: types.StringValue("a"), Order: types.StringUnknown()},
		{Name: types.StringValue("b"), Order: types.StringValue("desc")},
		{Name: types.StringValue("c")},
	})
	if d.HasError() {
		t.Fatalf("building columns: %v", d)
	}

	plan := spannerTableIndexModel{
		Columns: columns,
		Unique:  types.BoolUnknown(),
	}
	resolved, diags := resolveUnknownIndexInherited(ctx, plan)
	if diags.HasError() {
		t.Fatalf("resolveUnknownIndexInherited: %v", diags)
	}

	if !resolved.Unique.IsNull() {
		t.Errorf("unknown unique resolved to %v, want null", resolved.Unique)
	}
	var cols []spannerTableIndexColumn
	if d := resolved.Columns.ElementsAs(ctx, &cols, false); d.HasError() {
		t.Fatalf("decoding resolved columns: %v", d)
	}
	if !cols[0].Order.IsNull() {
		t.Errorf("unknown order resolved to %v, want null", cols[0].Order)
	}
	if cols[1].Order.ValueString() != "desc" {
		t.Error("explicit order must survive resolution")
	}
	if !cols[2].Order.IsNull() {
		t.Error("null order must stay null")
	}
}
