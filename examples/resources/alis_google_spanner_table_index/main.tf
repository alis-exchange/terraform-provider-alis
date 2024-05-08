terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "0.0.2-alpha8"
    }
  }
}

provider "alis" {

}

resource "alis_google_spanner_table_index" "test" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  name     = "display_name_idx"
  columns = [
    {
      name  = "display_name",
      order = "asc",
    }
  ]
  unique = false
}

output "test_table" {
  value = alis_google_spanner_table_index.test
}


