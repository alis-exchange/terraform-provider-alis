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

resource "alis_bigtable_table" "test" {
  project       = "mentenova-db-prod-woi"
  instance_name = "default"
  name          = "mentenova-db-prod-woi-test"
}

resource "alis_bigtable_table" "test2" {
  project       = "mentenova-db-prod-woi"
  instance_name = "default"
  name          = "mentenova-db-prod-woi-test2"
}

output "test_table" {
  value = alis_bigtable_table.test
}
