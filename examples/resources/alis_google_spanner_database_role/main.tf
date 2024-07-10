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

resource "alis_google_spanner_database_role" "admin_role" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  role     = "admin"
}

output "test_role" {
  description = "The Role in the database"
  value       = alis_google_spanner_database_role.admin_role
}
