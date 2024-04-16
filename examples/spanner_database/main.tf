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

resource "alis_spanner_database" "test" {
  project                  = "mentenova-db-prod-woi"
  instance_name            = "default"
  name                     = "mentenova-db-prod-woi-test"
  dialect                  = "GoogleStandardSql"
  enable_drop_protection   = false
  version_retention_period = "1h"
}

output "test_table" {
  value = alis_spanner_database.test
}
