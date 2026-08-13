package provider_test

import (
	"fmt"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSpannerTableForeignKey_basic(t *testing.T) {
	env := acctest.Setup(t)
	const (
		usersTable  = "tftest_fk_users"
		ordersTable = "tftest_fk_orders"
		fkName      = "FK_tftest_orders_user"
	)

	tablesOnly := env.ProviderBlock() + fmt.Sprintf(`
resource "alis_google_spanner_table" "users" {
  project         = %[1]q
  instance        = %[2]q
  database        = %[3]q
  name            = %[4]q
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name = "email",
        type = "STRING",
        size = 255,
      },
    ]
  }
}

resource "alis_google_spanner_table" "orders" {
  project         = %[1]q
  instance        = %[2]q
  database        = %[3]q
  name            = %[5]q
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name     = "user_id",
        type     = "INT64",
        required = true,
      },
    ]
  }
}

`, env.Project, env.Instance, env.Database, usersTable, ordersTable)

	config := func(onDelete string) string {
		return tablesOnly + fmt.Sprintf(`
resource "alis_google_spanner_table_foreign_key" "test" {
  project           = %[1]q
  instance          = %[2]q
  database          = %[3]q
  table             = alis_google_spanner_table.orders.name
  name              = %[4]q
  column            = "user_id"
  referenced_table  = alis_google_spanner_table.users.name
  referenced_column = "id"
  on_delete         = %[5]q
}
`, env.Project, env.Instance, env.Database, fkName, onDelete)
	}

	foreignKeyGone := acctest.CheckNotFound("foreign key", fkName, func() error {
		_, err := env.Service.GetSpannerTableForeignKeyConstraint(t.Context(), env.DatabaseName+"/tables/"+ordersTable, fkName)
		return err
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             foreignKeyGone,
		Steps: []resource.TestStep{
			{
				Config: config("CASCADE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table_foreign_key.test", "name", fkName),
					resource.TestCheckResourceAttr("alis_google_spanner_table_foreign_key.test", "table", ordersTable),
					resource.TestCheckResourceAttr("alis_google_spanner_table_foreign_key.test", "referenced_table", usersTable),
					resource.TestCheckResourceAttr("alis_google_spanner_table_foreign_key.test", "on_delete", "CASCADE"),
				),
			},
			{
				// Every foreign-key attribute requires replacement.
				Config: config("NO ACTION"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table_foreign_key.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("alis_google_spanner_table_foreign_key.test", "on_delete", "NO ACTION"),
			},
			{
				ResourceName:                         "alis_google_spanner_table_foreign_key.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("%s/tables/%s/constraints/%s", env.DatabaseName, ordersTable, fkName),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				// Drop only the constraint, keeping both tables: this is what
				// proves Delete issues ALTER TABLE ... DROP CONSTRAINT.
				// CheckDestroy cannot, since it runs once the tables are gone.
				Config: tablesOnly,
				Check:  foreignKeyGone,
			},
		},
	})
}
