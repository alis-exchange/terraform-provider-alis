package provider_test

import (
	"fmt"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSpannerDatabaseSequence_basic(t *testing.T) {
	env := acctest.Setup(t)
	const (
		sequence        = "tftest_seq"
		renamedSequence = "tftest_seq_renamed"
	)

	config := func(name string, startWith int) string {
		return env.ProviderBlock() + fmt.Sprintf(`
resource "alis_google_spanner_database_sequence" "test" {
  project  = %q
  instance = %q
  database = %q
  sequence = %q
  options = {
    sequence_kind = "bit_reversed_positive"
    skip_range = {
      min = 1000
      max = 5000000
    }
    start_with_counter = %d
  }
}
`, env.Project, env.Instance, env.Database, name, startWith)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy: acctest.CheckNotFound("sequence", renamedSequence, func() error {
			_, err := env.Service.GetSpannerSequence(t.Context(), env.DatabaseName+"/sequences/"+renamedSequence)
			return err
		}),
		Steps: []resource.TestStep{
			{
				Config: config(sequence, 1000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_database_sequence.test", "sequence", sequence),
					resource.TestCheckResourceAttr(
						"alis_google_spanner_database_sequence.test",
						"options.sequence_kind",
						"bit_reversed_positive",
					),
					resource.TestCheckResourceAttr("alis_google_spanner_database_sequence.test", "options.skip_range.min", "1000"),
					resource.TestCheckResourceAttr("alis_google_spanner_database_sequence.test", "options.start_with_counter", "1000"),
				),
			},
			{
				// Changing options is an in-place update.
				Config: config(sequence, 2000),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_database_sequence.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("alis_google_spanner_database_sequence.test", "options.start_with_counter", "2000"),
			},
			{
				// Renaming requires replacement.
				Config: config(renamedSequence, 2000),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("alis_google_spanner_database_sequence.test", plancheck.ResourceActionReplace),
					},
				},
			},
			{
				ResourceName:                         "alis_google_spanner_database_sequence.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("%s/sequences/%s", env.DatabaseName, renamedSequence),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "sequence",
			},
		},
	})
}
