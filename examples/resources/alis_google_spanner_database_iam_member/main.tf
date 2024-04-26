terraform {
  required_providers {
    google = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "alis" {

}

resource "alis_google_spanner_database_iam_member" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  role     = "roles/editor"
  member   = "serviceAccount:${var.ALIS_OS_SERVICE_ACCOUNT}"
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = alis_google_spanner_database_iam_member.editor
}
