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

data "alis_google_spanner_database_iam_policy" "policy" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = data.alis_google_spanner_database_iam_policy.policy
}