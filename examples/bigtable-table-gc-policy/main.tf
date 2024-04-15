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

resource "alis_bigtable_gc_policy" "test" {
  project         = "mentenova-db-prod-woi"
  instance_name   = "default"
  table           = "mentenova-db-prod-woi-test"
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
  value = alis_bigtable_gc_policy.test
}
