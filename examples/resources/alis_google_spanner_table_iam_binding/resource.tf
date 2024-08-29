resource "alis_google_spanner_table_iam_binding" "editor" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  role     = "admin"
  permissions = [
    "SELECT",
    "UPDATE",
    "INSERT",
    "DELETE",
  ]
}
