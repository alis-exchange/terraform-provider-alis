terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "0.0.1-alpha8"
    }
  }
}

provider "alis" {

}

resource "alis_google_bigtable_gc_policy" "test" {
  project       = var.ALIS_OS_PROJECT
  instance      = var.ALIS_OS_BIGTABLE_INSTANCE
  table         = "tf-test"
  column_family = "0"
  gc_rules      = <<EOF
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
