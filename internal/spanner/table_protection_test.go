package spanner

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The prior states these tests run against. Attributes absent from the JSON
// decode as null, so a minimal document is enough to drive the protection
// checks: schema, interleave and timeouts play no part in them.
const (
	protectedTableJSON   = `{"name":"t","project":"p","instance":"i","database":"d","prevent_destroy":true}`
	unprotectedTableJSON = `{"name":"t","project":"p","instance":"i","database":"d","prevent_destroy":false}`
	unsetProtectionJSON  = `{"name":"t","project":"p","instance":"i","database":"d"}`
	protectedTableName   = "projects/p/instances/i/databases/d/tables/t"
)

// tableState decodes JSON into prior state for the current table schema, the
// way the framework decodes state arriving over the wire.
func tableState(t *testing.T, stateJSON string) tfsdk.State {
	t.Helper()
	sch := resourceSchema(t, NewSpannerTableResource())
	raw, err := (&tfprotov6.RawState{JSON: []byte(stateJSON)}).UnmarshalWithOpts(
		sch.Type().TerraformType(context.Background()),
		tfprotov6.UnmarshalOpts{ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true}},
	)
	if err != nil {
		t.Fatalf("decoding table state: %v", err)
	}
	return tfsdk.State{Raw: raw, Schema: sch}
}

// nullTableValue is the value of a table that does not exist: prior state on
// create, and planned state on destroy.
func nullTableValue(t *testing.T) tftypes.Value {
	t.Helper()
	sch := resourceSchema(t, NewSpannerTableResource())
	return tftypes.NewValue(sch.Type().TerraformType(context.Background()), nil)
}

// diagText joins diagnostics into one string for substring assertions.
func diagText(diags diag.Diagnostics) string {
	parts := make([]string, 0, len(diags))
	for _, d := range diags {
		parts = append(parts, d.Summary()+": "+d.Detail())
	}
	return strings.Join(parts, "\n")
}

// assertProtectionError checks the diagnostics carry exactly one protection
// error, naming the table and attributed to attrPath.
func assertProtectionError(t *testing.T, diags diag.Diagnostics, summary string, attrPath path.Path) {
	t.Helper()
	errs := diags.Errors()
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %s", len(errs), diagText(diags))
	}
	if errs[0].Summary() != summary {
		t.Errorf("summary = %q, want %q", errs[0].Summary(), summary)
	}
	detail := errs[0].Detail()
	if !strings.Contains(detail, "protected from deletion") {
		t.Errorf("detail = %q, want it to mention protection from deletion", detail)
	}
	if !strings.Contains(detail, protectedTableName) {
		t.Errorf("detail = %q, want it to name the table", detail)
	}
	withPath, ok := errs[0].(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("error is not attributed to an attribute: %v", errs[0])
	}
	if !withPath.Path().Equal(attrPath) {
		t.Errorf("path = %s, want %s", withPath.Path(), attrPath)
	}
}

func TestGuardTableDestroy(t *testing.T) {
	ctx := context.Background()

	t.Run("protected state is refused", func(t *testing.T) {
		diags := guardTableDestroy(ctx, tableState(t, protectedTableJSON))
		assertProtectionError(t, diags, "Table Protected From Deletion", path.Root("prevent_destroy"))
	})

	t.Run("unprotected state passes", func(t *testing.T) {
		if diags := guardTableDestroy(ctx, tableState(t, unprotectedTableJSON)); diags.HasError() {
			t.Errorf("unexpected error: %s", diagText(diags))
		}
	})

	// An imported table records no value until its first apply.
	t.Run("unset protection passes", func(t *testing.T) {
		if diags := guardTableDestroy(ctx, tableState(t, unsetProtectionJSON)); diags.HasError() {
			t.Errorf("unexpected error: %s", diagText(diags))
		}
	})

	t.Run("state without a schema passes", func(t *testing.T) {
		if diags := guardTableDestroy(ctx, tfsdk.State{}); diags.HasError() {
			t.Errorf("unexpected error: %s", diagText(diags))
		}
	})
}

func TestGuardTableReplace(t *testing.T) {
	ctx := context.Background()
	attrPath := path.Root("name")

	t.Run("protected state is refused", func(t *testing.T) {
		diags := guardTableReplace(ctx, tableState(t, protectedTableJSON), attrPath)
		assertProtectionError(t, diags, "Table Protected From Replacement", attrPath)
		if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, "`name`") {
			t.Errorf("detail = %q, want it to name the changed attribute", detail)
		}
	})

	t.Run("unprotected state passes", func(t *testing.T) {
		if diags := guardTableReplace(ctx, tableState(t, unprotectedTableJSON), attrPath); diags.HasError() {
			t.Errorf("unexpected error: %s", diagText(diags))
		}
	})

	t.Run("state without a schema passes", func(t *testing.T) {
		if diags := guardTableReplace(ctx, tfsdk.State{}, attrPath); diags.HasError() {
			t.Errorf("unexpected error: %s", diagText(diags))
		}
	})
}

// runTableModifyPlan drives the resource's ModifyPlan the way the framework
// does: a null planned value is a destroy, a null prior state is a create.
func runTableModifyPlan(t *testing.T, state tfsdk.State, planRaw, configRaw tftypes.Value) *resource.ModifyPlanResponse {
	t.Helper()
	r, ok := NewSpannerTableResource().(resource.ResourceWithModifyPlan)
	if !ok {
		t.Fatal("spannerTableResource does not implement resource.ResourceWithModifyPlan")
	}

	sch := resourceSchema(t, NewSpannerTableResource())
	plan := tfsdk.Plan{Raw: planRaw, Schema: sch}
	resp := &resource.ModifyPlanResponse{Plan: plan, RequiresReplace: path.Paths{}}
	r.ModifyPlan(context.Background(), resource.ModifyPlanRequest{
		Config: tfsdk.Config{Raw: configRaw, Schema: sch},
		State:  state,
		Plan:   plan,
	}, resp)

	return resp
}

func TestSpannerTableResourceModifyPlan(t *testing.T) {
	protected := tableState(t, protectedTableJSON)
	unprotected := tableState(t, unprotectedTableJSON)
	unset := tableState(t, unsetProtectionJSON)
	nullValue := nullTableValue(t)

	t.Run("protected destroy is refused before apply", func(t *testing.T) {
		resp := runTableModifyPlan(t, protected, nullValue, nullValue)
		assertProtectionError(t, resp.Diagnostics, "Table Protected From Deletion", path.Root("prevent_destroy"))
		// The framework rejects a destroy plan whose planned state is not null.
		if !resp.Plan.Raw.IsNull() {
			t.Error("planned state was modified; a destroy plan must stay null")
		}
		if len(resp.RequiresReplace) != 0 {
			t.Errorf("RequiresReplace = %v, want none", resp.RequiresReplace)
		}
	})

	// Protection is read from prior state, so a configuration that lifts it in
	// the same change cannot bypass the check.
	t.Run("protected destroy with unprotected config is refused", func(t *testing.T) {
		resp := runTableModifyPlan(t, protected, nullValue, unprotected.Raw)
		assertProtectionError(t, resp.Diagnostics, "Table Protected From Deletion", path.Root("prevent_destroy"))
	})

	t.Run("unprotected destroy passes", func(t *testing.T) {
		resp := runTableModifyPlan(t, unprotected, nullValue, nullValue)
		if resp.Diagnostics.HasError() {
			t.Errorf("unexpected error: %s", diagText(resp.Diagnostics))
		}
	})

	t.Run("unset protection destroy passes", func(t *testing.T) {
		resp := runTableModifyPlan(t, unset, nullValue, nullValue)
		if resp.Diagnostics.HasError() {
			t.Errorf("unexpected error: %s", diagText(resp.Diagnostics))
		}
	})

	t.Run("protected update passes", func(t *testing.T) {
		resp := runTableModifyPlan(t, protected, protected.Raw, protected.Raw)
		if resp.Diagnostics.HasError() {
			t.Errorf("unexpected error: %s", diagText(resp.Diagnostics))
		}
	})

	t.Run("create passes", func(t *testing.T) {
		creating := tfsdk.State{Raw: nullValue, Schema: protected.Schema}
		resp := runTableModifyPlan(t, creating, protected.Raw, protected.Raw)
		if resp.Diagnostics.HasError() {
			t.Errorf("unexpected error: %s", diagText(resp.Diagnostics))
		}
	})
}

// A nil provider config is safe here: the guard returns before the Spanner
// service is reached. The unprotected path needs a service and is covered by
// the acceptance tests.
func TestSpannerTableResourceDeleteGuard(t *testing.T) {
	resp := &resource.DeleteResponse{}
	(&spannerTableResource{}).Delete(context.Background(), resource.DeleteRequest{
		State: tableState(t, protectedTableJSON),
	}, resp)

	assertProtectionError(t, resp.Diagnostics, "Table Protected From Deletion", path.Root("prevent_destroy"))
}

// interleaveObject builds an interleave value for the given parent table.
func interleaveObject(t *testing.T, parentTable string) types.Object {
	t.Helper()
	obj, d := types.ObjectValue(
		map[string]attr.Type{"parent_table": types.StringType, "on_delete": types.StringType},
		map[string]attr.Value{
			"parent_table": types.StringValue(parentTable),
			"on_delete":    types.StringValue("CASCADE"),
		},
	)
	if d.HasError() {
		t.Fatalf("building interleave object: %v", d)
	}
	return obj
}

// replaceOutcome is what one attribute's plan modifiers decided.
type replaceOutcome struct {
	diags    diag.Diagnostics
	replaces bool
}

// TestSpannerTableSchemaReplaceGuards drives the plan modifiers that are wired
// into the schema, so it covers both the guards and the wiring. Every
// RequiresReplaceIf wrapper skips its handler on create, on destroy, and when
// the value is unchanged, so each request carries a non-null plan and a
// changed value.
func TestSpannerTableSchemaReplaceGuards(t *testing.T) {
	ctx := context.Background()
	sch := resourceSchema(t, NewSpannerTableResource())

	schemaAttr, ok := sch.Attributes["schema"].(rschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("schema is %T, want SingleNestedAttribute", sch.Attributes["schema"])
	}
	columnsAttr, ok := schemaAttr.Attributes["columns"].(rschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("schema.columns is %T, want ListNestedAttribute", schemaAttr.Attributes["columns"])
	}
	interleaveAttr, ok := sch.Attributes["interleave"].(rschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("interleave is %T, want SingleNestedAttribute", sch.Attributes["interleave"])
	}

	typeChanged := minimalColumnModel()
	typeChanged.Type = types.StringValue("INT64")

	cases := map[string]struct {
		attrPath path.Path
		run      func(state tfsdk.State) replaceOutcome
	}{}

	for _, name := range []string{"name", "project", "instance", "database"} {
		stringAttr, ok := sch.Attributes[name].(rschema.StringAttribute)
		if !ok {
			t.Fatalf("%s is %T, want StringAttribute", name, sch.Attributes[name])
		}
		cases[name] = struct {
			attrPath path.Path
			run      func(state tfsdk.State) replaceOutcome
		}{
			attrPath: path.Root(name),
			run: func(state tfsdk.State) replaceOutcome {
				resp := &planmodifier.StringResponse{PlanValue: types.StringValue("changed")}
				for _, m := range stringAttr.PlanModifiers {
					m.PlanModifyString(ctx, planmodifier.StringRequest{
						Path:       path.Root(name),
						Config:     tfsdk.Config(state),
						Plan:       tfsdk.Plan(state),
						State:      state,
						StateValue: types.StringValue("t"),
						PlanValue:  types.StringValue("changed"),
					}, resp)
				}
				return replaceOutcome{resp.Diagnostics, resp.RequiresReplace}
			},
		}
	}

	cases["interleave"] = struct {
		attrPath path.Path
		run      func(state tfsdk.State) replaceOutcome
	}{
		attrPath: path.Root("interleave"),
		run: func(state tfsdk.State) replaceOutcome {
			planned := interleaveObject(t, "parent_two")
			resp := &planmodifier.ObjectResponse{PlanValue: planned}
			for _, m := range interleaveAttr.PlanModifiers {
				m.PlanModifyObject(ctx, planmodifier.ObjectRequest{
					Path:       path.Root("interleave"),
					Config:     tfsdk.Config(state),
					Plan:       tfsdk.Plan(state),
					State:      state,
					StateValue: interleaveObject(t, "parent_one"),
					PlanValue:  planned,
				}, resp)
			}
			return replaceOutcome{resp.Diagnostics, resp.RequiresReplace}
		},
	}

	cases["schema.columns"] = struct {
		attrPath path.Path
		run      func(state tfsdk.State) replaceOutcome
	}{
		attrPath: path.Root("schema").AtName("columns"),
		run: func(state tfsdk.State) replaceOutcome {
			planned := columnList(t, []spannerTableColumn{typeChanged})
			resp := &planmodifier.ListResponse{PlanValue: planned}
			for _, m := range columnsAttr.PlanModifiers {
				m.PlanModifyList(ctx, planmodifier.ListRequest{
					Path:       path.Root("schema").AtName("columns"),
					Config:     tfsdk.Config(state),
					Plan:       tfsdk.Plan(state),
					State:      state,
					StateValue: columnList(t, []spannerTableColumn{minimalColumnModel()}),
					PlanValue:  planned,
				}, resp)
			}
			return replaceOutcome{resp.Diagnostics, resp.RequiresReplace}
		},
	}

	for name, tc := range cases {
		t.Run(name+" refuses a replace while protected", func(t *testing.T) {
			got := tc.run(tableState(t, protectedTableJSON))
			assertProtectionError(t, got.diags, "Table Protected From Replacement", tc.attrPath)
		})

		t.Run(name+" replaces while unprotected", func(t *testing.T) {
			got := tc.run(tableState(t, unprotectedTableJSON))
			if got.diags.HasError() {
				t.Errorf("unexpected error: %s", diagText(got.diags))
			}
			if !got.replaces {
				t.Error("RequiresReplace = false, want true")
			}
		})
	}
}

// TestSpannerTableSchemaReplaceModifiers pins which attributes force a replace.
// It cannot tell a guarded modifier from an unguarded one, which is what
// TestSpannerTableSchemaReplaceGuards covers.
func TestSpannerTableSchemaReplaceModifiers(t *testing.T) {
	sch := resourceSchema(t, NewSpannerTableResource())
	schemaAttr, ok := sch.Attributes["schema"].(rschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("schema is %T, want SingleNestedAttribute", sch.Attributes["schema"])
	}
	interleaveAttr, ok := sch.Attributes["interleave"].(rschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("interleave is %T, want SingleNestedAttribute", sch.Attributes["interleave"])
	}

	replacing := map[string]any{
		"name":                    sch.Attributes["name"],
		"project":                 sch.Attributes["project"],
		"instance":                sch.Attributes["instance"],
		"database":                sch.Attributes["database"],
		"interleave":              sch.Attributes["interleave"],
		"schema.columns":          schemaAttr.Attributes["columns"],
		"interleave.parent_table": interleaveAttr.Attributes["parent_table"],
		"interleave.on_delete":    interleaveAttr.Attributes["on_delete"],
	}
	for name, attribute := range replacing {
		count := 0
		for _, desc := range planModifierDescriptions(t, attribute) {
			if strings.Contains(desc, "destroy and recreate") {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%s has %d replace modifiers, want exactly 1", name, count)
		}
	}

	for _, desc := range planModifierDescriptions(t, sch.Attributes["prevent_destroy"]) {
		if strings.Contains(desc, "destroy and recreate") {
			t.Error("prevent_destroy must not force a replace; lifting protection is an in-place update")
		}
	}
}
