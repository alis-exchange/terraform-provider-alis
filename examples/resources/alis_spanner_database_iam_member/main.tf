terraform {
  required_providers {
    google = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "google" {
  host = "localhost:8080"
}

resource "alis_spanner_database_iam_member" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  role     = "roles/bigtable.user"
  member   = "user:jane@example.com"
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = alis_spanner_database_iam_member.editor
}
