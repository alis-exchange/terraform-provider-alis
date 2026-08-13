package spanner

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Every resource in this provider funnels into blocking Spanner DDL, so each
// must expose a timeouts block letting users bound Create/Update/Delete. The
// enabled-ops set is pinned here: a block that silently drops an op would
// accept the config attribute but never apply it.
func TestAllResourceSchemas_HaveTimeoutsBlock(t *testing.T) {
	resources := map[string]resource.Resource{
		"table":       NewSpannerTableResource(),
		"index":       NewSpannerTableIndexResource(),
		"foreign_key": NewTableForeignKeyResource(),
		"ttl_policy":  NewTableTtlPolicyResource(),
		"iam_binding": NewTableIamBindingResource(),
		"role":        NewDatabaseRoleResource(),
		"sequence":    NewDatabaseSequenceResource(),
	}

	for name, r := range resources {
		t.Run(name, func(t *testing.T) {
			resp := &resource.SchemaResponse{}
			r.Schema(context.Background(), resource.SchemaRequest{}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
			}

			blk, ok := resp.Schema.Blocks["timeouts"]
			if !ok {
				t.Fatal("schema has no timeouts block")
			}

			typ, ok := blk.Type().(interface {
				AttributeTypes() map[string]attr.Type
			})
			if !ok {
				t.Fatalf("timeouts block type %T does not expose attribute types", blk.Type())
			}

			ops := typ.AttributeTypes()
			for _, op := range []string{"create", "update", "delete"} {
				if _, ok := ops[op]; !ok {
					t.Errorf("timeouts block missing %q", op)
				}
			}
			if len(ops) != 3 {
				t.Errorf("timeouts block has ops %v, want exactly create/update/delete", ops)
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	ctx, cancel := withTimeout(context.Background(), 0)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Error("zero duration must leave ctx unbounded")
	}

	ctx, cancel = withTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Error("positive duration must set a deadline")
	}
}
