terraform {
  required_providers {
    bigtable = {
      source = "alis.exchange/db/alis"
    }
  }
}

provider "alis" {
  host     = "localhost:8080"
}

data "alis_bigtable_tables" "test" {}

output "test_tables" {
  value = data.alis_bigtable_tables.test
}