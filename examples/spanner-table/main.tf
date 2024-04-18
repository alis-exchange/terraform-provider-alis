terraform {
  required_providers {
    bigtable = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "alis" {
  host = "localhost:8080"
}

resource "alis_spanner_table" "test" {
  project                  = "mentenova-db-prod-woi"
  instance_name            = "default"
  name                     = "mentenova-db-prod-woi-test"
  dialect                  = "GOOGLE_STANDARD_SQL"
  enable_drop_protection   = false
  version_retention_period = "3600s"
}

output "test_table" {
  value = alis_spanner_table.test
}
