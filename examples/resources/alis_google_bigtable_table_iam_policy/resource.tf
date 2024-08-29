resource "alis_google_bigtable_table_iam_policy" "policy" {
  project  = var.GOOGLE_PROJECT
  instance = var.BIGTABLE_INSTANCE
  table    = "tf-test"
  bindings = [
    {
      role = "roles/bigtable.user",
      members = [
        "serviceAccount:${var.SERVICE_ACCOUNT}"
      ]
    }
  ]
}