package spanner

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// All released provider versions before the Version 2 stamp (v1.x and the
// 2.0.0 betas) wrote state at schema version 0, so every resource must carry
// a version-0 upgrader or Terraform errors the moment it reads old state.
func upgradeTestResources() map[string]resource.Resource {
	return map[string]resource.Resource{
		"table":       NewSpannerTableResource(),
		"index":       NewSpannerTableIndexResource(),
		"foreign_key": NewTableForeignKeyResource(),
		"ttl_policy":  NewTableTtlPolicyResource(),
		"iam_binding": NewTableIamBindingResource(),
		"role":        NewDatabaseRoleResource(),
		"sequence":    NewDatabaseSequenceResource(),
	}
}

func resourceSchema(t *testing.T, r resource.Resource) rschema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// runUpgradeV0 mirrors how the framework's fwserver invokes a version-0
// StateUpgrader: prior state JSON is decoded against the upgrader's
// PriorSchema with unknown JSON fields ignored and absent attributes filled
// as null, and the response state starts empty with the current schema.
func runUpgradeV0(t *testing.T, r resource.Resource, stateJSON []byte) (tfsdk.State, diag.Diagnostics) {
	t.Helper()
	ctx := context.Background()

	withUpgrade, ok := r.(resource.ResourceWithUpgradeState)
	if !ok {
		t.Fatalf("%T does not implement ResourceWithUpgradeState", r)
	}
	upgrader, ok := withUpgrade.UpgradeState(ctx)[0]
	if !ok {
		t.Fatalf("%T has no StateUpgrader for prior schema version 0", r)
	}

	req := resource.UpgradeStateRequest{
		RawState: &tfprotov6.RawState{JSON: stateJSON},
	}
	if upgrader.PriorSchema != nil {
		raw, err := req.RawState.UnmarshalWithOpts(
			upgrader.PriorSchema.Type().TerraformType(ctx),
			tfprotov6.UnmarshalOpts{ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true}},
		)
		if err != nil {
			t.Fatalf("decoding prior state with PriorSchema: %v", err)
		}
		req.State = &tfsdk.State{Raw: raw, Schema: *upgrader.PriorSchema}
	}

	resp := &resource.UpgradeStateResponse{
		State: tfsdk.State{Schema: resourceSchema(t, r)},
	}
	upgrader.StateUpgrader(ctx, req, resp)
	if resp.DynamicValue != nil {
		t.Fatal("upgrader set DynamicValue; these tests only support the State path")
	}
	return resp.State, resp.Diagnostics
}

func TestAllResourceSchemas_VersionAndV0Upgrader(t *testing.T) {
	for name, r := range upgradeTestResources() {
		t.Run(name, func(t *testing.T) {
			if v := resourceSchema(t, r).Version; v != 2 {
				t.Errorf("schema Version = %d, want 2", v)
			}
			withUpgrade, ok := r.(resource.ResourceWithUpgradeState)
			if !ok {
				t.Fatalf("%T does not implement ResourceWithUpgradeState", r)
			}
			if _, ok := withUpgrade.UpgradeState(context.Background())[0]; !ok {
				t.Error("no StateUpgrader registered for prior schema version 0")
			}
		})
	}
}

// State captured from a real `terraform apply` of registry provider v1.5.2
// against the Spanner emulator: columns carry the removed attributes
// (auto_increment, unique, precision, scale, file_descriptor) and the
// interleave/timeouts fields do not exist yet.
func TestSpannerTableUpgradeState_FromV152(t *testing.T) {
	ctx := context.Background()
	stateJSON, err := os.ReadFile("testdata/table_state_v152.json")
	if err != nil {
		t.Fatal(err)
	}

	state, diags := runUpgradeV0(t, NewSpannerTableResource(), stateJSON)
	if diags.HasError() {
		t.Fatalf("upgrade errored: %v", diags)
	}

	var m spannerTableModel
	if d := state.Get(ctx, &m); d.HasError() {
		t.Fatalf("upgraded state does not fit current model: %v", d)
	}

	if got := m.Name.ValueString(); got != "tftest_upgrade" {
		t.Errorf("name = %q, want tftest_upgrade", got)
	}
	if got := m.Database.ValueString(); got != "test-database" {
		t.Errorf("database = %q", got)
	}
	if !m.PreventDestroy.ValueBool() {
		t.Error("prevent_destroy lost in upgrade")
	}
	if m.Interleave != nil {
		t.Errorf("interleave = %+v, want nil (absent in v1.5.2 state)", m.Interleave)
	}
	if !m.Timeouts.IsNull() {
		t.Errorf("timeouts = %v, want null (absent in v1.5.2 state)", m.Timeouts)
	}

	var cols []spannerTableColumn
	if d := m.Schema.Columns.ElementsAs(ctx, &cols, false); d.HasError() {
		t.Fatalf("columns: %v", d)
	}
	if len(cols) != 7 {
		t.Fatalf("got %d columns, want 7", len(cols))
	}
	id := cols[0]
	if id.Name.ValueString() != "id" || !id.IsPrimaryKey.ValueBool() || id.Type.ValueString() != "INT64" {
		t.Errorf("id column mangled: %+v", id)
	}
	if !id.IsStored.IsNull() {
		t.Errorf("id.is_stored = %v, want null", id.IsStored)
	}
	displayName := cols[1]
	if !displayName.Required.ValueBool() || displayName.Size.ValueInt64() != 255 {
		t.Errorf("display_name column mangled: %+v", displayName)
	}
	isActive := cols[4]
	if isActive.DefaultValue.ValueString() != "true" {
		t.Errorf("is_active.default_value = %q, want true", isActive.DefaultValue.ValueString())
	}
	updatedAt := cols[5]
	if !updatedAt.AutoUpdateTime.ValueBool() {
		t.Error("updated_at.auto_update_time lost in upgrade")
	}

	// The removed attributes must not vanish silently: auto_increment kept a
	// live sequence-backed DEFAULT on the column, unique users need to know
	// the index resource replaces it, and v1 quoted string defaults for the
	// user (DEFAULT ('hello')) while v2 emits default_value verbatim — an
	// unquoted literal fails DDL parsing on the next apply.
	var warnings []string
	for _, d := range diags {
		if d.Severity() == diag.SeverityWarning {
			warnings = append(warnings, d.Summary()+" "+d.Detail())
		}
	}
	if len(warnings) != 3 {
		t.Fatalf(
			"got %d warnings %v, want 3 (auto_increment on id, unique on email, default_value on str_default)",
			len(warnings),
			warnings,
		)
	}
	all := strings.Join(warnings, "\n")
	if !strings.Contains(all, "auto_increment") || !strings.Contains(all, `"id"`) {
		t.Errorf("missing auto_increment warning naming column id:\n%s", all)
	}
	if !strings.Contains(all, "unique") || !strings.Contains(all, `"email"`) {
		t.Errorf("missing unique warning naming column email:\n%s", all)
	}
	if !strings.Contains(all, `"str_default"`) || !strings.Contains(all, "'hello'") {
		t.Errorf("missing string default_value warning naming str_default with quoted example:\n%s", all)
	}
}

// The 2.0.0 betas also wrote schema version 0, but in the current shape:
// is_stored, interleave and timeouts may hold real values that the upgrade
// must carry over untouched, with no migration warnings.
func TestSpannerTableUpgradeState_FromBetaShape(t *testing.T) {
	ctx := context.Background()
	stateJSON, err := os.ReadFile("testdata/table_state_beta.json")
	if err != nil {
		t.Fatal(err)
	}

	state, diags := runUpgradeV0(t, NewSpannerTableResource(), stateJSON)
	if len(diags) != 0 {
		t.Fatalf("beta-shaped state must upgrade silently, got: %v", diags)
	}

	var m spannerTableModel
	if d := state.Get(ctx, &m); d.HasError() {
		t.Fatalf("upgraded state does not fit current model: %v", d)
	}

	if m.Interleave == nil || m.Interleave.ParentTable.ValueString() != "tftest_parent" ||
		m.Interleave.OnDelete.ValueString() != "CASCADE" {
		t.Errorf("interleave lost in upgrade: %+v", m.Interleave)
	}
	if m.Timeouts.IsNull() {
		t.Error("timeouts lost in upgrade")
	}

	var cols []spannerTableColumn
	if d := m.Schema.Columns.ElementsAs(ctx, &cols, false); d.HasError() {
		t.Fatalf("columns: %v", d)
	}
	score := cols[3]
	if score.Name.ValueString() != "score" || !score.IsStored.ValueBool() || !score.IsComputed.ValueBool() ||
		score.ComputationDdl.ValueString() != "id + 1" {
		t.Errorf("score computed-column fields lost in upgrade: %+v", score)
	}
}

// The remaining resources kept an identical attribute shape since v1.5.2
// (they only gained the timeouts block), so their version-0 upgrade is a
// pass-through: decoding old JSON against the current schema must be the
// exact upgraded state, with absent timeouts as null.
func TestPassthroughUpgradeState(t *testing.T) {
	priorStates := map[string]string{
		"index":       `{"columns":[{"name":"c1","order":"ASC"}],"database":"d","instance":"i","name":"idx1","project":"p","table":"t","unique":true}`,
		"foreign_key": `{"column":"c","database":"d","instance":"i","name":"fk1","on_delete":"CASCADE","project":"p","referenced_column":"rc","referenced_table":"rt","table":"t"}`,
		"ttl_policy":  `{"column":"c","database":"d","instance":"i","project":"p","table":"t","ttl":30}`,
		"iam_binding": `{"database":"d","instance":"i","permissions":["SELECT","INSERT"],"project":"p","role":"admin","table":"t"}`,
		"role":        `{"database":"d","instance":"i","project":"p","role":"admin"}`,
		"sequence":    `{"database":"d","instance":"i","options":{"sequence_kind":"bit_reversed_positive","skip_range":null,"start_with_counter":null},"project":"p","sequence":"my_seq"}`,
	}

	resources := upgradeTestResources()
	for name, stateJSON := range priorStates {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			r := resources[name]

			state, diags := runUpgradeV0(t, r, []byte(stateJSON))
			if len(diags) != 0 {
				t.Fatalf("pass-through upgrade must be silent, got: %v", diags)
			}

			want, err := (&tfprotov6.RawState{JSON: []byte(stateJSON)}).UnmarshalWithOpts(
				resourceSchema(t, r).Type().TerraformType(ctx),
				tfprotov6.UnmarshalOpts{ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true}},
			)
			if err != nil {
				t.Fatalf("decoding prior state with current schema: %v", err)
			}
			if !state.Raw.Equal(want) {
				t.Errorf("upgraded state diverged from prior state:\ngot:  %v\nwant: %v", state.Raw, want)
			}
		})
	}
}
