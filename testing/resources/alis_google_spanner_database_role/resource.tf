// Local manual verification for alis_google_spanner_database_role and the
// alis_google_spanner_database_roles data source.
resource "alis_google_spanner_database_role" "test_role" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  role     = "tf_test_role"
}

data "alis_google_spanner_database_roles" "all" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE

  depends_on = [alis_google_spanner_database_role.test_role]
}

// Expect tf_test_role to appear here after apply.
output "database_roles" {
  value = data.alis_google_spanner_database_roles.all.roles
}
