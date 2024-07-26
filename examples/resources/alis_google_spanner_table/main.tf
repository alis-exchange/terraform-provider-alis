terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 1.3.2"
    }
  }
}

provider "alis" {

}

resource "alis_google_spanner_table" "test" {
  project         = var.ALIS_OS_PROJECT
  instance        = var.ALIS_OS_SPANNER_INSTANCE
  database        = "tf-test"
  name            = "tftest"
  prevent_destroy = true
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
      },
      {
        name            = "proto_test",
        type            = "PROTO",
        proto_package   = "com.example.Message",
        file_descriptor = "gcs:gs://path/to/my/descriptorset.pb",
      },
      {
        name            = "computed_column",
        type            = "STRING",
        is_computed     = true,
        computation_ddl = "proto_test.example_field",
      },
      {
        name = "arr_str",
        type = "ARRAY<STRING>",
      },
      {
        name = "arr_int64",
        type = "ARRAY<INT64>",
      },
      {
        name = "arr_float32",
        type = "ARRAY<FLOAT32>",
      },
      {
        name = "arr_float64",
        type = "ARRAY<FLOAT64>",
      }
    ]
  }
}

output "test_table" {
  value = alis_google_spanner_table.test
}


