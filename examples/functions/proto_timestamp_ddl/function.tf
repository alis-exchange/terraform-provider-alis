# Generates:
# TIMESTAMP_ADD(TIMESTAMP_SECONDS(Book.create_time.seconds),INTERVAL CAST(FLOOR(Book.create_time.nanos / 1000) AS INT64) MICROSECOND)
output "create_time_ddl" {
  value = provider::alis::proto_timestamp_ddl("Book.create_time")
}

# Typical use: a stored computed TIMESTAMP column mirroring a proto Timestamp field.
resource "alis_google_spanner_table" "books" {
  project         = var.GOOGLE_PROJECT
  instance        = var.SPANNER_INSTANCE
  database        = var.SPANNER_DATABASE
  name            = "books"
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "key",
        type           = "STRING",
        is_primary_key = true,
        required       = true,
      },
      {
        name          = "Book",
        type          = "PROTO",
        proto_package = "com.example.Book",
        required      = true,
      },
      {
        name            = "create_time",
        type            = "TIMESTAMP",
        is_computed     = true,
        computation_ddl = provider::alis::proto_timestamp_ddl("Book.create_time"),
        is_stored       = true,
      },
    ]
  }
}
