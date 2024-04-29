terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 0.0.1-alpha4"
    }
  }
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
