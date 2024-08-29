resource "alis_google_spanner_database_iam_member" "editor" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  role     = "roles/editor"
  member   = "serviceAccount:${var.SERVICE_ACCOUNT}"
}