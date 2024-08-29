resource "alis_google_spanner_database_role" "admin_role" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  role     = "admin"
}