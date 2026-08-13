resource "alis_google_spanner_table" "test_table" {
  project         = var.GOOGLE_PROJECT
  instance        = var.SPANNER_INSTANCE
  database        = var.SPANNER_DATABASE
  name            = var.SPANNER_TABLE
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
