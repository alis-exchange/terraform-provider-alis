# Fails at plan when the role does not exist, so bindings that reference a
# role managed elsewhere can depend on it explicitly.
data "alis_google_spanner_database_role" "reader" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  role     = "reader"
}

resource "alis_google_spanner_table_iam_binding" "books_reader" {
  project     = var.GOOGLE_PROJECT
  instance    = var.SPANNER_INSTANCE
  database    = "tf-test"
  table       = "books"
  role        = data.alis_google_spanner_database_role.reader.role
  permissions = ["SELECT"]
}
