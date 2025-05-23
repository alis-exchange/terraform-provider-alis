resource "alis_google_spanner_table" "example" {
  project         = var.GOOGLE_PROJECT
  instance        = var.SPANNER_INSTANCE
  database        = "tf-test"
  name            = "tftest"
  prevent_destroy = true
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
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
        is_stored       = true,
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
  interleave = {
    parent_table = alis_google_spanner_table.other_example.name
    on_delete    = "CASCADE"
  }
}


