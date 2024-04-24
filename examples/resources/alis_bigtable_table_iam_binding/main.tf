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

resource "alis_bigtable_table_iam_binding" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_BIGTABLE_INSTANCE
  table    = "tf-test"
  role     = "roles/bigtable.user"
  members = [
    "user:jane@example.com",
  ]
}

output "test_iam" {
  description = "The IAM policy for the table"
  value       = alis_bigtable_table_iam_binding.editor
}
