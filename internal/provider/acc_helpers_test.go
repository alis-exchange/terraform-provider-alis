package provider_test

import (
	"fmt"

	"terraform-provider-alis/internal/acctest"
)

// baseTableConfig renders a minimal table for tests that hang child
// resources (indexes, foreign keys, TTL policies, IAM bindings) off a table.
// The Terraform resource label and the Spanner table name vary per test so
// configs can coexist in one database.
func baseTableConfig(env acctest.Env, label, table string) string {
	return fmt.Sprintf(`
resource "alis_google_spanner_table" %q {
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
        size = 255,
      },
      {
        name = "created_at",
        type = "TIMESTAMP",
      },
    ]
  }
}
`, label, env.Project, env.Instance, env.Database, table)
}
