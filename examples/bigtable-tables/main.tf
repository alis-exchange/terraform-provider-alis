terraform {
  required_providers {
    bigtable = {
      source = "alis.exchange/db/alisbuild"
    }
  }
}

provider "alisbuild" {
  host     = "localhost:8080"
}

data "alisbuild_tables" "test" {}

output "test_tables" {
  value = data.alisbuild_tables.test
}