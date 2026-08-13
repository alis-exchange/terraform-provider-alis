// Local manual verification for alis_google_spanner_table_index.
// Prerequisite: apply testing/resources/alis_google_spanner_table first —
// this index targets the tf_test table it creates in database "play".
resource "alis_google_spanner_table_index" "test_index" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  table    = var.SPANNER_TABLE
  name     = "tf_test_display_name_idx"
  columns = [
    {
      name  = "display_name",
      order = "asc",
    },
    {
      name  = "inception_date",
      order = "desc",
    }
  ]
  unique = false
}
