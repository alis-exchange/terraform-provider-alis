package spanner

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Spanner secondary indexes cannot be altered in place: any change to the
// index definition requires dropping and recreating the index. Every
// attribute of the index resource must therefore force a replace; without
// this, a changed attribute would plan as an in-place update that never
// reaches Spanner.
func TestSpannerTableIndexSchema_AllAttributesRequireReplace(t *testing.T) {
	resp := &resource.SchemaResponse{}
	(&spannerTableIndexResource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)

	for name, attr := range resp.Schema.Attributes {
		found := false
		// Every RequiresReplace modifier, regardless of attribute type,
		// shares this description; matching on it avoids depending on the
		// framework's unexported modifier types.
		for _, desc := range planModifierDescriptions(t, attr) {
			if strings.Contains(desc, "destroy and recreate") {
				found = true
			}
		}
		if !found {
			t.Errorf("attribute %q has no RequiresReplace plan modifier; index changes must recreate the index", name)
		}
	}
}

// planModifierDescriptions extracts the descriptions of an attribute's plan
// modifiers. The framework's typed modifier slices ([]planmodifier.String,
// []planmodifier.Bool, ...) share no common interface, so this reflects over
// the PlanModifiers field and asserts each element's Description method.
func planModifierDescriptions(t *testing.T, attr any) []string {
	t.Helper()
	field := reflect.ValueOf(attr).FieldByName("PlanModifiers")
	if !field.IsValid() {
		t.Fatalf("attribute %T has no PlanModifiers field", attr)
	}

	var out []string
	for i := range field.Len() {
		m, ok := field.Index(i).Interface().(interface {
			Description(context.Context) string
		})
		if !ok {
			t.Fatalf("plan modifier %v lacks a Description method", field.Index(i))
		}
		out = append(out, m.Description(context.Background()))
	}
	return out
}
