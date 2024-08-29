resource "alis_google_spanner_database_iam_binding" "editor" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  role     = "roles/editor"
  members = [
    "serviceAccount:${var.SERVICE_ACCOUNT}",
  ]
}