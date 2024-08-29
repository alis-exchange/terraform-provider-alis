resource "alis_google_spanner_table_index" "test" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  name     = "display_name_idx"
  columns = [
    {
      name  = "display_name",
      order = "asc",
    }
  ]
  unique = false
}

