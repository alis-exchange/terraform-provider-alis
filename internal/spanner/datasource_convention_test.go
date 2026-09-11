package spanner

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// Every data source in this provider follows one shape: the attributes that
// identify the object are Required, everything else is Computed, and there is
// no timeouts block (reads are not long-running DDL). Pinning it here keeps a
// new data source from drifting into optional-with-default or resource-only
// attributes.
func TestAllDataSourceSchemas_FollowConvention(t *testing.T) {
	cases := []struct {
		name     string
		ds       datasource.DataSource
		identity []string
	}{
		{"database_roles", NewDatabaseRolesDataSource(), []string{"project", "instance", "database"}},
		{"table_iam_binding", NewTableIamBindingDataSource(), []string{"project", "instance", "database", "table", "role"}},
		{"database_role", NewDatabaseRoleDataSource(), []string{"project", "instance", "database", "role"}},
		{"table_ttl_policy", NewTableTTLPolicyDataSource(), []string{"project", "instance", "database", "table"}},
		{"table_foreign_key", NewTableForeignKeyDataSource(), []string{"project", "instance", "database", "table", "name"}},
		{"table_index", NewTableIndexDataSource(), []string{"project", "instance", "database", "table", "name"}},
		{"database_sequence", NewDatabaseSequenceDataSource(), []string{"project", "instance", "database", "sequence"}},
		{"table", NewSpannerTableDataSource(), []string{"project", "instance", "database", "name"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &datasource.SchemaResponse{}
			tc.ds.Schema(context.Background(), datasource.SchemaRequest{}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
			}
			if _, ok := resp.Schema.Blocks["timeouts"]; ok {
				t.Error("data source must not expose a timeouts block")
			}
			for name, attr := range resp.Schema.Attributes {
				if slices.Contains(tc.identity, name) {
					if !attr.IsRequired() {
						t.Errorf("identity attribute %q must be Required", name)
					}
					continue
				}
				assertComputedOnly(t, name, attr)
			}
		})
	}
}

// assertComputedOnly checks attr and, for nested attributes, every attribute
// beneath it is Computed and neither Required nor Optional.
func assertComputedOnly(t *testing.T, name string, attr schema.Attribute) {
	t.Helper()
	if !attr.IsComputed() || attr.IsRequired() || attr.IsOptional() {
		t.Errorf("output attribute %q must be Computed only (computed=%v required=%v optional=%v)",
			name, attr.IsComputed(), attr.IsRequired(), attr.IsOptional())
	}
	var nested map[string]schema.Attribute
	switch a := attr.(type) {
	case schema.SingleNestedAttribute:
		nested = a.Attributes
	case schema.ListNestedAttribute:
		nested = a.NestedObject.Attributes
	case schema.SetNestedAttribute:
		nested = a.NestedObject.Attributes
	case schema.MapNestedAttribute:
		nested = a.NestedObject.Attributes
	}
	for childName, child := range nested {
		assertComputedOnly(t, name+"."+childName, child)
	}
}
