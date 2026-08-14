# Generates:
# DATE(CAST((Book.publish_date).year AS INT64),CAST((Book.publish_date).month AS INT64),CAST((Book.publish_date).day AS INT64))
output "publish_date_ddl" {
  value = provider::alis::proto_date_ddl("Book.publish_date")
}

# Typical use: a stored computed DATE column mirroring a proto google.type.Date field.
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
        name            = "publish_date",
        type            = "DATE",
        is_computed     = true,
        computation_ddl = provider::alis::proto_date_ddl("Book.publish_date"),
        is_stored       = true,
      },
    ]
  }
}
