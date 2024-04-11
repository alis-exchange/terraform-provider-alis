terraform {
  required_providers {
    bigtable = {
      source = "alis.exchange/db/alisbuild"
    }
  }
}

provider "alisbuild" {}

data "alisbuild_tables" "example" {}
