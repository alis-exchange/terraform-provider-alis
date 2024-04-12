terraform {
  required_providers {
    bigtable = {
      source = "alis.exchange/db/alis"
    }
  }
}

provider "alis" {}

data "alis_bigtable_tables" "example" {}
