package provider_test

import (
	"fmt"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSpannerTableIamBinding_basic(t *testing.T) {
	env := acctest.Setup(t)
	env.SkipIfNotLive(t, "emulator does not surface INFORMATION_SCHEMA.TABLE_PRIVILEGES, which binding reads require")
	const (
		table = "tftest_iam_table"
		role  = "tftest_admin"
	)

	tableAndRole := env.ProviderBlock() + baseTableConfig(env, "base", table) + fmt.Sprintf(`
resource "alis_google_spanner_database_role" "grantee" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q
  role     = %[4]q
}
`, env.Project, env.Instance, env.Database, role)

	config := func(permissions string) string {
		return tableAndRole + fmt.Sprintf(`
resource "alis_google_spanner_table_iam_binding" "test" {
  project     = %[1]q
  instance    = %[2]q
  database    = %[3]q
  table       = alis_google_spanner_table.base.name
  role        = alis_google_spanner_database_role.grantee.role
  permissions = [%[5]s]
}

data "alis_google_spanner_table_iam_binding" "test" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q
  table    = alis_google_spanner_table.base.name
  role     = alis_google_spanner_database_role.grantee.role

  depends_on = [alis_google_spanner_table_iam_binding.test]
}
`, env.Project, env.Instance, env.Database, role, permissions)
	}

	parent := env.DatabaseName + "/tables/" + table
	bindingGone := acctest.CheckNotFound("IAM binding for role", role, func() error {
		_, err := env.Service.GetTableIamBinding(t.Context(), parent, role)
		return err
	})

	// Read the privileges Spanner actually holds, not the ones state claims:
	// an update that grants without revoking leaves the two disagreeing.
	grantedPermissions := func(want int) func(*terraform.State) error {
		return func(*terraform.State) error {
			binding, err := env.Service.GetTableIamBinding(t.Context(), parent, role)
			if err != nil {
				return fmt.Errorf("reading IAM binding for %q: %w", role, err)
			}
			if len(binding.Permissions) != want {
				return fmt.Errorf(
					"role %q holds %d permissions in Spanner, want %d: %v",
					role,
					len(binding.Permissions),
					want,
					binding.Permissions,
				)
			}

			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             bindingGone,
		Steps: []resource.TestStep{
			{
				Config: config(`"SELECT", "INSERT", "UPDATE", "DELETE"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table_iam_binding.test", "role", role),
					resource.TestCheckResourceAttr("alis_google_spanner_table_iam_binding.test", "permissions.#", "4"),
					resource.TestCheckResourceAttr("data.alis_google_spanner_table_iam_binding.test", "permissions.#", "4"),
					grantedPermissions(4),
				),
			},
			{
				// Dropping a permission is an in-place update, and it must
				// REVOKE: a grant-only update would leave DELETE in place and
				// state permanently out of step with the database.
				Config: config(`"SELECT", "INSERT", "UPDATE"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table_iam_binding.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table_iam_binding.test", "permissions.#", "3"),
					grantedPermissions(3),
				),
			},
			{
				ResourceName:                         "alis_google_spanner_table_iam_binding.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("%s/tables/%s/tableRoles/%s", env.DatabaseName, table, role),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "role",
			},
			{
				// Drop only the binding, keeping its table and role: this is
				// what proves Delete issues REVOKE. CheckDestroy cannot, since
				// it runs once the table is gone.
				Config: tableAndRole,
				Check:  bindingGone,
			},
		},
	})
}
