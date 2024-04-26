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

resource "alis_google_bigtable_table_iam_binding" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_BIGTABLE_INSTANCE
  table    = "tf-test"
  role     = "roles/bigtable.user"
  members  = [
    "serviceAccount:${var.ALIS_OS_SERVICE_ACCOUNT}"
  ]
}

output "test_iam" {
  description = "The IAM policy for the table"
  value       = alis_google_bigtable_table_iam_binding.editor
}
