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

data "alis_google_spanner_database_roles" "roles" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
}

output "test_iam" {
  description = "The Roles in the Spanner Database."
  value       = data.alis_google_spanner_database_roles.roles
}