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

resource "alis_google_spanner_database" "test" {
  project                  = var.ALIS_OS_PROJECT
  instance                 = var.ALIS_OS_SPANNER_INSTANCE
  name                     = "tf-test"
  dialect                  = "GOOGLE_STANDARD_SQL"
  enable_drop_protection   = false
  version_retention_period = "3600s"
}

output "test_table" {
  value = alis_google_spanner_database.test
}
