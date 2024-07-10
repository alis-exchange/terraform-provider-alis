terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "1.1.2"
    }
  }
}

provider "alis" {

}

resource "alis_google_spanner_table_iam_binding" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  role     = "admin"
  permissions = [
    "SELECT",
    "UPDATE",
    "INSERT",
    "DELETE",
  ]
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = alis_google_spanner_table_iam_binding.editor
}
