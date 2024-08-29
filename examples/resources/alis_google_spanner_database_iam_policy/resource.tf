resource "alis_google_spanner_database_iam_policy" "policy" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  bindings = [
    {
      role = "roles/editor",
      members = [
        "serviceAccount:${var.SERVICE_ACCOUNT}",
      ]
    }
  ]
}