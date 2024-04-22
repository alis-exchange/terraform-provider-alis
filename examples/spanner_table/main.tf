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

resource "alis_spanner_table" "test" {
  project       = "mentenova-db-prod-woi"
  instance_name = "default"
  database_name = "mentenova-db-prod-woi-test"
  name          = "portfolios"
  schema        = {
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
        name = "inception_date",
        type = "DATE",
      },
      {
        name = "last_refreshed_at",
        type = "TIMESTAMP",
      }
    ],
    indices = [
      {
        name    = "display_name_idx",
        columns = ["display_name"],
        unique  = true,
      },
      {
        name    = "inception_date_idx",
        columns = ["inception_date", "last_refreshed_at"],
        unique  = false,
      }
    ]
  }
}

output "test_table" {
  value = alis_spanner_table.test
}
