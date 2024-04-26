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

resource "alis_google_spanner_table" "test" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  name     = "tftest"
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        unique         = true,
        required       = true,
      },
      {
        name = "display_name",
        type = "STRING",
        size = 255,
      },
      {
        name = "is_active",
        type = "BOOL",
      },
      {
        name          = "latest_return",
        type          = "FLOAT64",
        default_value = 0.0,
      },
      {
        name          = "earliest_return",
        type          = "FLOAT64",
        default_value = 0.0,
      },
      {
        name = "inception_date",
        type = "DATE",
      },
      {
        name = "last_refreshed_at",
        type = "TIMESTAMP",
      },
      {
        name = "metadata",
        type = "JSON",
      },
      {
        name = "data",
        type = "BYTES",
      }
    ],
    indices = [
      {
        name    = "display_name_idx",
        columns = ["display_name"],
        unique  = false,
      },
    ]
  }
}

output "test_table" {
  value = alis_google_spanner_table.test
}
