package provider_test

import (
	"fmt"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Provider-defined functions require Terraform 1.8+; skip cleanly on older
// binaries rather than failing on the provider:: call syntax.
var functionVersionChecks = []tfversion.TerraformVersionCheck{
	tfversion.SkipBelow(tfversion.Version1_8_0),
}

func TestAccSpannerFunctions_ddlOutputs(t *testing.T) {
	env := acctest.Setup(t)

	config := env.ProviderBlock() + `
output "proto_timestamp" {
  value = provider::alis::proto_timestamp_ddl("Book.create_time")
}

output "proto_date" {
  value = provider::alis::proto_date_ddl("Book.publish_date")
}

output "ancestor_single" {
  value = provider::alis::resource_name_ancestor_ddl("Book.name", "shelves")
}

output "ancestor_nested" {
  value = provider::alis::resource_name_ancestor_ddl("Book.name", "shelves", "books")
}

output "resource_id" {
  value = provider::alis::resource_name_id_ddl("Book.name", "books")
}
`

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   functionVersionChecks,
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput(
						"proto_timestamp",
						"TIMESTAMP_ADD(TIMESTAMP_SECONDS(Book.create_time.seconds),INTERVAL CAST(FLOOR(Book.create_time.nanos / 1000) AS INT64) MICROSECOND)",
					),
					resource.TestCheckOutput(
						"proto_date",
						"DATE(CAST((Book.publish_date).year AS INT64),CAST((Book.publish_date).month AS INT64),CAST((Book.publish_date).day AS INT64))",
					),
					resource.TestCheckOutput("ancestor_single",
						"REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+)')"),
					resource.TestCheckOutput("ancestor_nested",
						"REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+/books/[^/]+)')"),
					resource.TestCheckOutput("resource_id",
						"REGEXP_EXTRACT(Book.name, r'books/([^/]+)')"),
				),
			},
		},
	})
}

// The generated expressions must round-trip verbatim through
// INFORMATION_SCHEMA.GENERATION_EXPRESSION, or every refresh would show a
// permanent diff on the computed column. The framework's post-apply
// idempotency plan proves that here.
func TestAccSpannerFunctions_computedColumnRoundTrip(t *testing.T) {
	env := acctest.Setup(t)
	const table = "tftest_fn_computed"

	config := env.ProviderBlock() + fmt.Sprintf(`
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
        name = "name",
        type = "STRING",
      },
      {
        name            = "parent",
        type            = "STRING",
        is_computed     = true,
        computation_ddl = provider::alis::resource_name_ancestor_ddl("name", "shelves"),
        is_stored       = true,
      },
      {
        name            = "book_id",
        type            = "STRING",
        is_computed     = true,
        computation_ddl = provider::alis::resource_name_id_ddl("name", "books"),
        is_stored       = true,
      },
    ]
  }
}
`, env.Project, env.Instance, env.Database, table)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   functionVersionChecks,
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             checkTableDestroy(env, t, table),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.2.computation_ddl",
						"REGEXP_EXTRACT(name, r'^(shelves/[^/]+)')"),
					resource.TestCheckResourceAttr("alis_google_spanner_table.test", "schema.columns.3.computation_ddl",
						"REGEXP_EXTRACT(name, r'books/([^/]+)')"),
				),
			},
		},
	})
}
