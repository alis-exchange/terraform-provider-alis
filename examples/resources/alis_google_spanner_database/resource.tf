resource "alis_google_spanner_database" "test" {
  project                  = var.GOOGLE_PROJECT
  instance                 = var.SPANNER_INSTANCE
  name                     = "tf-test"
  dialect                  = "GOOGLE_STANDARD_SQL"
  enable_drop_protection   = false
  version_retention_period = "3600s"
}