data "alis_google_spanner_table_iam_binding" "admin_binding" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  role     = "admin"
}