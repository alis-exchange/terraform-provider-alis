package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// tableConfig renders a table whose display_name STRING size and optional
// extra column vary per step, so steps exercise in-place updates.
func tableConfig(env acctest.Env, name string, displayNameSize int, withNotes bool) string {
	notes := ""
	if withNotes {
		notes = `
      {
        name = "notes",
        type = "STRING",
        size = 100,
      },`
	}

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
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name = "display_name",
        type = "STRING",
        size = %d,
      },
      {
        name = "is_active",
        type = "BOOL",
      },
      {
        name = "created_at",
        type = "TIMESTAMP",
      },%s
    ]
  }
}
`, env.Project, env.Instance, env.Database, name, displayNameSize, notes)
}

// checkTableDestroy asserts the table is gone from the backend after destroy.
func checkTableDestroy(env acctest.Env, t *testing.T, table string) resource.TestCheckFunc {
	return acctest.CheckNotFound("table", table, func() error {
		_, err := env.Service.GetSpannerTable(t.Context(), env.DatabaseName+"/tables/"+table)
		return err
	})
}

func TestAccSpannerTable_basic(t *testing.T) {
	env := acctest.Setup(t)
	const table = "tftest_basic"

	// Import cannot restore prevent_destroy (config-only computed default).
	// The per-column booleans are ignored because hydration is asymmetric:
	// the create/update path stores config-omitted booleans as explicit
	// false, while the import path leaves them null.
	importIgnore := []string{"prevent_destroy"}
	for i := range 5 {
		importIgnore = append(importIgnore,
			fmt.Sprintf("schema.columns.%d.is_computed", i),
			fmt.Sprintf("schema.columns.%d.required", i),
		)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             checkTableDestroy(env, t, table),
		Steps: []resource.TestStep{
			{
				Config: tableConfig(env, table, 255, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "name", table),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "project", env.Project),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "instance", env.Instance),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "database", env.Database),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.#", "4"),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.0.name", "id"),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.0.is_primary_key", "true"),
				),
			},
			{
				// Growing a STRING size and adding a column are in-place updates.
				Config: tableConfig(env, table, 500, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.#", "5"),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.1.size", "500"),
				),
			},
			{
				ResourceName:      "alis_google_spanner_table.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/tables/%s", env.DatabaseName, table),
				ImportStateVerify: true,
				// Framework resources have no "id" attribute; compare on name.
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              importIgnore,
			},
		},
	})
}

func TestAccSpannerTable_replaceOnKeyChange(t *testing.T) {
	env := acctest.Setup(t)
	const table = "tftest_replace"

	config := func(pkOnValue bool) string {
		// The attribute is omitted rather than set to false: hydration
		// collapses an unset boolean back to null, and an explicit false
		// would show as phantom drift on refresh.
		pkLine := ""
		if pkOnValue {
			pkLine = `
        is_primary_key = true,`
		}
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
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name = "value",
        type = "INT64",%s
      },
    ]
  }
}
`, env.Project, env.Instance, env.Database, table, pkLine)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             checkTableDestroy(env, t, table),
		Steps: []resource.TestStep{
			{
				Config: config(false),
			},
			{
				// Changing the primary key cannot be done in place.
				Config: config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_table.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccSpannerTable_preventDestroyGuard(t *testing.T) {
	env := acctest.Setup(t)
	const table = "tftest_guarded"

	config := func(preventDestroyLine string) string {
		return env.ProviderBlock() + fmt.Sprintf(`
resource "alis_google_spanner_table" "test" {
  project  = %q
  instance = %q
  database = %q
  name     = %q
  %s
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
    ]
  }
}
`, env.Project, env.Instance, env.Database, table, preventDestroyLine)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             checkTableDestroy(env, t, table),
		Steps: []resource.TestStep{
			{
				// prevent_destroy defaults to true when omitted.
				Config: config(""),
				Check: resource.TestCheckResourceAttr(
					"alis_google_spanner_table.test", "prevent_destroy", "true",
				),
			},
			{
				Config:      config(""),
				Destroy:     true,
				ExpectError: regexp.MustCompile("protected from deletion"),
			},
			{
				// Lift the guard so the test's final destroy succeeds.
				Config: config("prevent_destroy = false"),
			},
		},
	})
}
