terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "0.0.1-alpha8"
    }
  }
}

provider "alis" {
}

data "alis_google_bigtable_table_iam_policy" "policy" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_BIGTABLE_INSTANCE
  table    = "tf-test"
}

output "test_iam" {
  description = "The IAM policy for the table"
  value       = data.alis_google_bigtable_table_iam_policy.policy
}
