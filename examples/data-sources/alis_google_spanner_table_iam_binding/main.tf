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

data "alis_google_spanner_table_iam_binding" "admin_binding" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  role     = "admin"
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = data.alis_google_spanner_table_iam_binding.admin_binding
}