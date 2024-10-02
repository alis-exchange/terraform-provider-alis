resource "alis_google_spanner_table_ttl_policy" "test" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  column   = "updated_at"
  ttl      = 1
}



