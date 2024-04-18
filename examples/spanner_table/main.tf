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
        nullable       = false,
      },
      {
        name     = "display_name",
        type     = "STRING",
        size     = 255,
        nullable = true,
      },
      {
        name     = "is_active",
        type     = "BOOL",
        nullable = true,
      },
      {
        name          = "latest_return",
        type          = "FLOAT64",
        nullable      = true,
        default_value = 0.0,
      },
      {
        name     = "inception_date",
        type     = "DATE",
        nullable = true,
      },
      {
        name     = "last_refreshed_at",
        type     = "TIMESTAMP",
        nullable = true,
      }
    ],
  }
}

output "test_table" {
  value = alis_spanner_table.test
}
