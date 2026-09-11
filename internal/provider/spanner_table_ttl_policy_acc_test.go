package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSpannerTableTTLPolicy_basic(t *testing.T) {
	env := acctest.Setup(t)
	const table = "tftest_ttl_table"

	config := func(ttlDays int) string {
		return env.ProviderBlock() + baseTableConfig(env, "base", table) + fmt.Sprintf(`
resource "alis_google_spanner_table_ttl_policy" "test" {
  project  = %q
  instance = %q
  database = %q
  table    = alis_google_spanner_table.base.name
  column   = "created_at"
  ttl      = %d
}
`, env.Project, env.Instance, env.Database, ttlDays)
	}

	policyGone := acctest.CheckNotFound("TTL policy on table", table, func() error {
		_, err := env.Service.GetSpannerTableRowDeletionPolicy(t.Context(), env.DatabaseName+"/tables/"+table)
		return err
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             policyGone,
		Steps: []resource.TestStep{
			{
				Config: config(30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table_ttl_policy.test", "table", table),
					resource.TestCheckResourceAttr("alis_google_spanner_table_ttl_policy.test", "column", "created_at"),
					resource.TestCheckResourceAttr("alis_google_spanner_table_ttl_policy.test", "ttl", "30"),
				),
			},
			{
				// Shrinking the TTL is an in-place update.
				Config: config(7),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table_ttl_policy.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("alis_google_spanner_table_ttl_policy.test", "ttl", "7"),
			},
			{
				// The data source reads the policy just updated.
				Config: config(7) + fmt.Sprintf(`
data "alis_google_spanner_table_ttl_policy" "read" {
  project  = %q
  instance = %q
  database = %q
  table    = alis_google_spanner_table.base.name

  depends_on = [alis_google_spanner_table_ttl_policy.test]
}
`, env.Project, env.Instance, env.Database),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.alis_google_spanner_table_ttl_policy.read", "column", "created_at"),
					resource.TestCheckResourceAttr("data.alis_google_spanner_table_ttl_policy.read", "ttl", "7"),
				),
			},
			{
				// A TTL policy is imported by its table's name.
				ResourceName:                         "alis_google_spanner_table_ttl_policy.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("%s/tables/%s", env.DatabaseName, table),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "table",
			},
			{
				// A table without a policy (here: one that does not exist) is an error.
				Config: config(7) + fmt.Sprintf(`
data "alis_google_spanner_table_ttl_policy" "missing" {
  project  = %q
  instance = %q
  database = %q
  table    = "tftest_no_such_table"
}
`, env.Project, env.Instance, env.Database),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Error Reading TTL Policy`),
			},
			{
				// Drop only the policy, keeping its table: this is what proves
				// Delete issues ALTER TABLE ... DROP ROW DELETION POLICY.
				// CheckDestroy cannot, since it runs once the table is gone.
				Config: env.ProviderBlock() + baseTableConfig(env, "base", table),
				Check:  policyGone,
			},
		},
	})
}
