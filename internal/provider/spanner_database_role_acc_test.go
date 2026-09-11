package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSpannerDatabaseRole_basic(t *testing.T) {
	env := acctest.Setup(t)
	env.SkipIfNoRoleListing(t)
	const role = "tftest_role"

	config := env.ProviderBlock() + fmt.Sprintf(`
resource "alis_google_spanner_database_role" "test" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q
  role     = %[4]q
}

data "alis_google_spanner_database_roles" "all" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q

  depends_on = [alis_google_spanner_database_role.test]
}
`, env.Project, env.Instance, env.Database, role)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy: acctest.CheckNotFound("role", role, func() error {
			_, err := env.Service.GetDatabaseRole(t.Context(), env.DatabaseName+"/databaseRoles/"+role)
			return err
		}),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_database_role.test", "role", role),
					// The data source stores full role resource names.
					resource.TestCheckTypeSetElemAttr(
						"data.alis_google_spanner_database_roles.all", "roles.*",
						env.DatabaseName+"/databaseRoles/"+role,
					),
				),
			},
			{
				// The singular data source reads the role just created.
				Config: config + fmt.Sprintf(`
data "alis_google_spanner_database_role" "read" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q
  role     = %[4]q

  depends_on = [alis_google_spanner_database_role.test]
}
`, env.Project, env.Instance, env.Database, role),
				Check: resource.TestCheckResourceAttr("data.alis_google_spanner_database_role.read", "role", role),
			},
			{
				ResourceName:                         "alis_google_spanner_database_role.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("%s/databaseRoles/%s", env.DatabaseName, role),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "role",
			},
			{
				// A missing role is an error, not an empty result.
				Config: config + fmt.Sprintf(`
data "alis_google_spanner_database_role" "missing" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q
  role     = "tftest_missing_role"
}
`, env.Project, env.Instance, env.Database),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Error Reading Database Role`),
			},
		},
	})
}
