terraform {
  required_providers {
    google = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "alis" {
  host = "localhost:8080"
}

resource "alis_bigtable_table" "test" {
  project         = var.ALIS_OS_PROJECT
  instance        = var.ALIS_OS_BIGTABLE_INSTANCE
  name            = "tf-test"
  column_families = [
    {
      name = "0"
    },
  ]
}

output "test_table" {
  value = alis_bigtable_table.test
}
