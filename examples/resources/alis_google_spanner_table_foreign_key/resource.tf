resource "alis_google_spanner_table_foreign_key" "test" {
  project           = var.GOOGLE_PROJECT
  instance          = var.SPANNER_INSTANCE
  database          = "tf-test"
  table             = "tftest"
  name              = "FK_user_key"
  column            = "user"
  referenced_table  = "users"
  referenced_column = "id"
  on_delete         = "CASCADE"
}




