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

data "alis_spanner_database_iam_policy" "policy" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tftest"
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = data.alis_spanner_database_iam_policy.policy
}