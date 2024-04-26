terraform {
  required_providers {
    alis = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "alis" {

}

resource "alis_google_bigtable_table" "test" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_BIGTABLE_INSTANCE
  name     = "tf-test"
  column_families = [
    {
      name = "0"
    },
  ]
  deletion_protection = false
  #  change_stream_retention = "86400s"
}

output "test_table" {
  value = alis_google_bigtable_table.test
}
