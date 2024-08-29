resource "alis_google_bigtable_table_iam_binding" "editor" {
  project  = var.GOOGLE_PROJECT
  instance = var.BIGTABLE_INSTANCE
  table    = "tf-test"
  role     = "roles/bigtable.user"
  members = [
    "serviceAccount:${var.SERVICE_ACCOUNT}"
  ]
}