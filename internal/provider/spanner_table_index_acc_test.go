package provider_test

import (
	"fmt"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSpannerTableIndex_basic(t *testing.T) {
	env := acctest.Setup(t)
	const (
		table = "tftest_idx_table"
		index = "tftest_display_name_idx"
	)

	config := func(twoColumns bool) string {
		columns := `
    {
      name  = "display_name",
      order = "asc",
    },`
		if twoColumns {
			columns += `
    {
      name  = "created_at",
      order = "desc",
    },`
		}
		return env.ProviderBlock() + baseTableConfig(env, "base", table) + fmt.Sprintf(`
resource "alis_google_spanner_table_index" "test" {
  project  = %q
  instance = %q
  database = %q
  table    = alis_google_spanner_table.base.name
  name     = %q
  columns = [%s
  ]
  unique = false
}
`, env.Project, env.Instance, env.Database, index, columns)
	}

	indexGone := acctest.CheckNotFound("index", index, func() error {
		_, err := env.Service.GetSpannerTableIndex(t.Context(), env.DatabaseName+"/tables/"+table, index)
		return err
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             indexGone,
		Steps: []resource.TestStep{
			{
				Config: config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table_index.test", "name", index),
					resource.TestCheckResourceAttr("alis_google_spanner_table_index.test", "table", table),
					resource.TestCheckResourceAttr("alis_google_spanner_table_index.test", "columns.#", "2"),
					resource.TestCheckResourceAttr("alis_google_spanner_table_index.test", "columns.1.order", "desc"),
					resource.TestCheckResourceAttr("alis_google_spanner_table_index.test", "unique", "false"),
				),
			},
			{
				// Every index attribute requires replacement.
				Config: config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table_index.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("alis_google_spanner_table_index.test", "columns.#", "1"),
			},
			{
				ResourceName:                         "alis_google_spanner_table_index.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("%s/tables/%s/indexes/%s", env.DatabaseName, table, index),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				// Drop only the index. CheckDestroy runs after the table is
				// gone, when the index reports NotFound either way; this step
				// is what actually proves Delete issues DROP INDEX.
				Config: env.ProviderBlock() + baseTableConfig(env, "base", table),
				Check:  indexGone,
			},
		},
	})
}
