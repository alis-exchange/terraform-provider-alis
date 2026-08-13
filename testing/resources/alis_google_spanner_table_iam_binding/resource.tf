// Local manual verification for alis_google_spanner_table_iam_binding.
// Prerequisite: apply testing/resources/alis_google_spanner_table first —
// the binding grants on the tf_test table in database "play".
// Creates its own database role so the GRANT has a grantee.
resource "alis_google_spanner_database_role" "tf_test_admin" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  role     = "tf_test_admin"
}

resource "alis_google_spanner_table_iam_binding" "test_binding" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  table    = var.SPANNER_TABLE
  role     = alis_google_spanner_database_role.tf_test_admin.role
  permissions = [
    "SELECT",
    "INSERT",
    "UPDATE",
    "DELETE",
  ]

  depends_on = [alis_google_spanner_database_role.tf_test_admin]
}
