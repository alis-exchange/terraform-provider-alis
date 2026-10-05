package provider_test

import (
	"fmt"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// columnOrderConfig renders a table whose computed owner column is named
// ownerColumn and sits before created_at in the config. Renaming it is an
// in-place drop and add, so Spanner appends the new column after created_at.
func columnOrderConfig(env acctest.Env, name, ownerColumn string, payloadSize int) string {
	return env.ProviderBlock() + fmt.Sprintf(`
resource "alis_google_spanner_table" "test" {
  project         = %q
  instance        = %q
  database        = %q
  name            = %q
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "key",
        type           = "STRING",
        size           = 64,
        is_primary_key = true,
        required       = true,
      },
      {
        name = "payload",
        type = "STRING",
        size = %d,
      },
      {
        name            = %q,
        type            = "STRING",
        is_computed     = true,
        computation_ddl = "UPPER(payload)",
        is_stored       = true,
      },
      {
        name = "created_at",
        type = "TIMESTAMP",
      },
    ]
  }
}
`, env.Project, env.Instance, env.Database, name, payloadSize, ownerColumn)
}

// TestAccSpannerTable_columnReplacedInPlaceConverges covers a column replaced
// in place: Spanner cannot reorder columns, so the live order no longer
// matches the config, and that alone must not produce a plan diff.
func TestAccSpannerTable_columnReplacedInPlaceConverges(t *testing.T) {
	env := acctest.Setup(t)
	const table = "tftest_column_order"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             checkTableDestroy(env, t, table),
		Steps: []resource.TestStep{
			{
				Config: columnOrderConfig(env, table, "owner", 100),
			},
			{
				// The framework's post-apply plan must be empty here, even
				// though the live order is now key, payload, created_at, owner_key.
				Config: columnOrderConfig(env, table, "owner_key", 100),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.2.name", "owner_key"),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.3.name", "created_at"),
				),
			},
			{
				Config: columnOrderConfig(env, table, "owner_key", 100),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				// A real column change on the reordered table is still detected.
				Config: columnOrderConfig(env, table, "owner_key", 200),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.1.size", "200"),
			},
		},
	})
}
