terraform {
  required_providers {
    google = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "alis" {

}

resource "alis_google_bigtable_gc_policy" "test" {
  project         = var.ALIS_OS_PROJECT
  instance        = var.ALIS_OS_BIGTABLE_INSTANCE
  table           = "tf-test"
  column_family   = "0"
  deletion_policy = "ABANDON"
  gc_rules        = <<EOF
  {
    "rules": [
      {
        "max_version": 10
      }
    ]
  }
  EOF
}

output "test_table" {
  value = alis_google_bigtable_gc_policy.test
}
