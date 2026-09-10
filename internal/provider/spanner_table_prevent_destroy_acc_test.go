package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Both patterns pin the refusal to the plan phase, which is the whole point of
// the guard: an apply that has already destroyed other resources cannot be
// rolled back. The harness wraps a plan failure in one of these two phrases
// depending on whether the step applies, and Terraform wraps diagnostic text
// for non-TTY output, so the words are matched across whitespace.
var (
	// A PlanOnly step runs only "terraform plan -refresh=false [-destroy]".
	planRejectsProtected = regexp.MustCompile(`(?s)Error running non-refresh plan.*protected\s+from\s+deletion`)
	// A step that would apply fails in its pre-apply plan, so apply never runs.
	preApplyRejectsProtected = regexp.MustCompile(`(?s)Error running pre-apply plan.*protected\s+from\s+deletion`)
)

// pdTable describes one table block for the prevent_destroy tests. The zero
// value renders a protected table with an INT64 primary key "id" and a
// nullable INT64 "value" column.
type pdTable struct {
	label string
	// name may embed HCL interpolation such as ${count.index}.
	name string
	// unprotected renders prevent_destroy = false; omitting the attribute
	// leaves it at its default of true.
	unprotected bool
	// count renders a count meta-argument when positive.
	count int
	// valueType overrides the type of the "value" column (INT64 when empty).
	valueType string
	// valueIsPK promotes "value" to a primary key column, a change Spanner
	// cannot apply in place.
	valueIsPK bool
	// withNotes adds a nullable STRING column, an in-place update.
	withNotes bool
}

func pdTableConfig(env acctest.Env, o pdTable) string {
	prevent, count, valuePK, notes := "", "", "", ""
	if o.unprotected {
		prevent = "\n  prevent_destroy = false"
	}
	if o.count > 0 {
		count = fmt.Sprintf("\n  count = %d", o.count)
	}
	if o.valueIsPK {
		valuePK = "\n        is_primary_key = true,"
	}
	if o.withNotes {
		notes = `
      {
        name = "notes",
        type = "STRING",
        size = 100,
      },`
	}
	valueType := o.valueType
	if valueType == "" {
		valueType = "INT64"
	}

	return fmt.Sprintf(`
resource "alis_google_spanner_table" %q {%s
  project  = %q
  instance = %q
  database = %q
  name     = %q%s
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
        type = %q,%s
      },%s
    ]
  }
}
`, o.label, count, env.Project, env.Instance, env.Database, o.name, prevent, valueType, valuePK, notes)
}

// checkTableExists is the counterpart of checkTableDestroy, for steps that must
// leave a table untouched. A rejected plan is only worth something if nothing
// was destroyed, and Check funcs never run on a PlanOnly step, so the
// assertions belong on the no-op apply that follows.
func checkTableExists(env acctest.Env, t *testing.T, table string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if _, err := env.Service.GetSpannerTable(t.Context(), env.DatabaseName+"/tables/"+table); err != nil {
			return fmt.Errorf("table %q should still exist: %w", table, err)
		}
		return nil
	}
}

func expectActions(actions map[string]plancheck.ResourceActionType) resource.ConfigPlanChecks {
	checks := make([]plancheck.PlanCheck, 0, len(actions))
	for address, action := range actions {
		checks = append(checks, plancheck.ExpectResourceAction(address, action))
	}
	return resource.ConfigPlanChecks{PreApply: checks}
}

func TestAccSpannerTable_preventDestroyGuard(t *testing.T) {
	env := acctest.Setup(t)
	const (
		table   = "tftest_guarded"
		renamed = "tftest_guarded_renamed"
		addr    = "alis_google_spanner_table.test"
	)

	config := func(o pdTable) string {
		o.label = "test"
		return env.ProviderBlock() + pdTableConfig(env, o)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			checkTableDestroy(env, t, table),
			checkTableDestroy(env, t, renamed),
		),
		Steps: []resource.TestStep{
			{
				// prevent_destroy defaults to true when omitted. Creating a
				// protected table, and the harness's follow-up no-op plans,
				// are unaffected by the guard.
				Config: config(pdTable{name: table}),
				Check:  resource.TestCheckResourceAttr(addr, "prevent_destroy", "true"),
			},
			{
				// An explicit destroy is refused while planning.
				Config:      config(pdTable{name: table}),
				Destroy:     true,
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// The guard reads the value recorded in state, so lifting
				// protection in the same change as the destroy is still
				// refused: the change has not been applied yet.
				Config:      config(pdTable{name: table, unprotected: true}),
				Destroy:     true,
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// The full destroy path fails in its pre-apply plan, so Delete
				// never runs and nothing is destroyed.
				Config:      config(pdTable{name: table}),
				Destroy:     true,
				ExpectError: preApplyRejectsProtected,
			},
			{
				// In-place schema changes stay allowed while protected. This
				// step also proves the refused plans above left the table.
				Config:           config(pdTable{name: table, withNotes: true}),
				ConfigPlanChecks: expectActions(map[string]plancheck.ResourceActionType{addr: plancheck.ResourceActionUpdate}),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, table),
					resource.TestCheckResourceAttr(addr, "prevent_destroy", "true"),
					resource.TestCheckResourceAttr(addr, "schema.columns.#", "3"),
				),
			},
			{
				// Lifting the guard on its own is an in-place update.
				Config:           config(pdTable{name: table, withNotes: true, unprotected: true}),
				ConfigPlanChecks: expectActions(map[string]plancheck.ResourceActionType{addr: plancheck.ResourceActionUpdate}),
				Check:            resource.TestCheckResourceAttr(addr, "prevent_destroy", "false"),
			},
			{
				// Unprotected, an identity change replaces the table.
				Config:           config(pdTable{name: renamed, withNotes: true, unprotected: true}),
				ConfigPlanChecks: expectActions(map[string]plancheck.ResourceActionType{addr: plancheck.ResourceActionReplace}),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableDestroy(env, t, table),
					checkTableExists(env, t, renamed),
					resource.TestCheckResourceAttr(addr, "name", renamed),
				),
			},
			{
				// Unprotected, an explicit destroy succeeds.
				Config:  config(pdTable{name: renamed, withNotes: true, unprotected: true}),
				Destroy: true,
				Check:   checkTableDestroy(env, t, renamed),
			},
		},
	})
}

// TestAccSpannerTable_preventDestroyMixed runs a protected table alongside an
// unprotected one, so a refused plan has something to leave intact, and covers
// each way the provider itself decides to replace a table.
func TestAccSpannerTable_preventDestroyMixed(t *testing.T) {
	env := acctest.Setup(t)
	const (
		guarded     = "tftest_pd_guarded"
		open        = "tftest_pd_open"
		guardedAddr = "alis_google_spanner_table.guarded"
		openAddr    = "alis_google_spanner_table.open"
	)

	// config always renders the unprotected table, plus the given variant of
	// the protected one; omitting the variant removes it from configuration.
	config := func(guardedVariant ...pdTable) string {
		cfg := env.ProviderBlock() + pdTableConfig(env, pdTable{label: "open", name: open, unprotected: true})
		for _, g := range guardedVariant {
			g.label = "guarded"
			cfg += pdTableConfig(env, g)
		}
		return cfg
	}
	both := config(pdTable{name: guarded})
	onlyOpen := config()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			checkTableDestroy(env, t, guarded),
			checkTableDestroy(env, t, open),
		),
		Steps: []resource.TestStep{
			{
				Config: both,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(guardedAddr, "prevent_destroy", "true"),
					resource.TestCheckResourceAttr(openAddr, "prevent_destroy", "false"),
				),
			},
			{
				// Destroying everything fails at plan, so the unprotected
				// table is not destroyed on the way to the protected one.
				Config:      both,
				Destroy:     true,
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// Removing only the protected resource from configuration is
				// an orphan destroy, refused at plan.
				Config:      onlyOpen,
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// A change to the table identity would replace it.
				Config:      config(pdTable{name: guarded + "_renamed"}),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// So would a column type change.
				Config:      config(pdTable{name: guarded, valueType: "FLOAT64"}),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// So would promoting a column to a primary key.
				Config:      config(pdTable{name: guarded, valueIsPK: true}),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// Replacements read the value recorded in state, so bundling
				// the protection change with the replacement is refused.
				Config:      config(pdTable{name: guarded, unprotected: true, valueType: "FLOAT64"}),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// Check funcs never run on a PlanOnly step, so this no-op
				// apply is what proves every refused plan above left both
				// tables and the recorded state untouched.
				Config: both,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, guarded),
					checkTableExists(env, t, open),
					resource.TestCheckResourceAttr(guardedAddr, "name", guarded),
					resource.TestCheckResourceAttr(guardedAddr, "prevent_destroy", "true"),
					resource.TestCheckResourceAttr(guardedAddr, "schema.columns.1.type", "INT64"),
					resource.TestCheckResourceAttr(openAddr, "name", open),
				),
			},
			{
				Config: config(pdTable{name: guarded, unprotected: true}),
				ConfigPlanChecks: expectActions(
					map[string]plancheck.ResourceActionType{guardedAddr: plancheck.ResourceActionUpdate},
				),
				Check: resource.TestCheckResourceAttr(guardedAddr, "prevent_destroy", "false"),
			},
			{
				// Once the protection change is applied, the replacement goes
				// through.
				Config: config(pdTable{name: guarded, unprotected: true, valueType: "FLOAT64"}),
				ConfigPlanChecks: expectActions(
					map[string]plancheck.ResourceActionType{guardedAddr: plancheck.ResourceActionReplace},
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, guarded),
					resource.TestCheckResourceAttr(guardedAddr, "schema.columns.1.type", "FLOAT64"),
				),
			},
			{
				// The orphan destroy now succeeds and leaves the other table.
				Config: onlyOpen,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableDestroy(env, t, guarded),
					checkTableExists(env, t, open),
				),
			},
		},
	})
}

// TestAccSpannerTable_preventDestroyInterleave covers the interleave block,
// whose every change replaces the table. Steps that execute DDL keep
// on_delete = CASCADE: that renders INTERLEAVE IN PARENT, while an unset or
// NO ACTION on_delete renders the plain INTERLEAVE IN form, so the change to
// NO ACTION is asserted at plan only.
func TestAccSpannerTable_preventDestroyInterleave(t *testing.T) {
	env := acctest.Setup(t)
	const (
		parent    = "tftest_pd_parent"
		parentTwo = "tftest_pd_parent_two"
		child     = "tftest_pd_child"
		childAddr = "alis_google_spanner_table.child"
	)

	config := func(parentLabel, onDelete string, unprotected bool) string {
		prevent := ""
		if unprotected {
			prevent = "\n  prevent_destroy = false"
		}

		return env.ProviderBlock() +
			pdTableConfig(env, pdTable{label: "parent", name: parent, unprotected: true}) +
			pdTableConfig(env, pdTable{label: "parent_two", name: parentTwo, unprotected: true}) +
			fmt.Sprintf(`
resource "alis_google_spanner_table" "child" {
  project  = %[1]q
  instance = %[2]q
  database = %[3]q
  name     = %[4]q%[7]s
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name           = "child_id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
    ]
  }
  interleave = {
    parent_table = alis_google_spanner_table.%[5]s.name
    on_delete    = %[6]q
  }
}
`, env.Project, env.Instance, env.Database, child, parentLabel, onDelete, prevent)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			checkTableDestroy(env, t, child),
			checkTableDestroy(env, t, parent),
			checkTableDestroy(env, t, parentTwo),
		),
		Steps: []resource.TestStep{
			{
				Config: config("parent", "CASCADE", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(childAddr, "prevent_destroy", "true"),
					resource.TestCheckResourceAttr(childAddr, "interleave.parent_table", parent),
					resource.TestCheckResourceAttr(childAddr, "interleave.on_delete", "CASCADE"),
				),
			},
			{
				// Changing the delete action would replace the table.
				Config:      config("parent", "NO ACTION", false),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				// So would moving the child under a different parent.
				Config:      config("parent_two", "CASCADE", false),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				Config: config("parent", "CASCADE", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, parent),
					checkTableExists(env, t, parentTwo),
					checkTableExists(env, t, child),
					resource.TestCheckResourceAttr(childAddr, "interleave.parent_table", parent),
				),
			},
			{
				Config: config("parent", "CASCADE", true),
				ConfigPlanChecks: expectActions(
					map[string]plancheck.ResourceActionType{childAddr: plancheck.ResourceActionUpdate},
				),
				Check: resource.TestCheckResourceAttr(childAddr, "prevent_destroy", "false"),
			},
			{
				Config: config("parent_two", "CASCADE", true),
				ConfigPlanChecks: expectActions(
					map[string]plancheck.ResourceActionType{childAddr: plancheck.ResourceActionReplace},
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, child),
					resource.TestCheckResourceAttr(childAddr, "interleave.parent_table", parentTwo),
				),
			},
		},
	})
}

// TestAccSpannerTable_preventDestroyCount covers a reduced count, which
// destroys the instances that fall outside the new range without any
// configuration block being removed.
func TestAccSpannerTable_preventDestroyCount(t *testing.T) {
	env := acctest.Setup(t)
	const (
		first  = "tftest_pd_count_0"
		second = "tftest_pd_count_1"
	)

	config := func(count int, unprotected bool) string {
		return env.ProviderBlock() + pdTableConfig(env, pdTable{
			label:       "test",
			name:        "tftest_pd_count_${count.index}",
			count:       count,
			unprotected: unprotected,
		})
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			checkTableDestroy(env, t, first),
			checkTableDestroy(env, t, second),
		),
		Steps: []resource.TestStep{
			{
				Config: config(2, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table.test.0", "name", first),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test.1", "name", second),
				),
			},
			{
				// Reducing the count destroys the last instance.
				Config:      config(1, false),
				PlanOnly:    true,
				ExpectError: planRejectsProtected,
			},
			{
				Config: config(2, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, first),
					checkTableExists(env, t, second),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test.1", "name", second),
				),
			},
			{
				Config: config(2, true),
				// plancheck addresses instances the way the plan JSON does.
				ConfigPlanChecks: expectActions(map[string]plancheck.ResourceActionType{
					"alis_google_spanner_table.test[0]": plancheck.ResourceActionUpdate,
					"alis_google_spanner_table.test[1]": plancheck.ResourceActionUpdate,
				}),
				Check: resource.TestCheckResourceAttr("alis_google_spanner_table.test.1", "prevent_destroy", "false"),
			},
			{
				Config: config(1, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkTableExists(env, t, first),
					checkTableDestroy(env, t, second),
				),
			},
		},
	})
}
