resource "alis_google_bigtable_table_iam_member" "editor" {
  project  = var.GOOGLE_PROJECT
  instance = var.BIGTABLE_INSTANCE
  table    = "tf-test"
  role     = "roles/bigtable.user"
  member   = "serviceAccount:${var.SERVICE_ACCOUNT}"
}