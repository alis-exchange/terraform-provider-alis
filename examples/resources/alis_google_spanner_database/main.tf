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
