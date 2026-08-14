# The generated expressions, visible with terraform plan / terraform output.
output "proto_timestamp_ddl" {
  value = provider::alis::proto_timestamp_ddl("Book.create_time")
}

output "ancestor_ddl" {
  value = provider::alis::resource_name_ancestor_ddl("Book.name", "shelves")
}

output "nested_ancestor_ddl" {
  value = provider::alis::resource_name_ancestor_ddl("Book.name", "shelves", "books")
}

output "id_ddl" {
  value = provider::alis::resource_name_id_ddl("Book.name", "books")
}

# A real table whose computed columns come from the functions; a second
# plan after apply must be empty, proving the generated DDL round-trips
# INFORMATION_SCHEMA unchanged.
resource "alis_google_spanner_table" "test_functions" {
  project         = var.GOOGLE_PROJECT
  instance        = var.SPANNER_INSTANCE
  database        = var.SPANNER_DATABASE
  name            = "test_functions"
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "key",
        type           = "STRING",
        size           = 64,
        is_primary_key = true,
        required       = true,
      },
      {
        name = "name",
        type = "STRING",
      },
      {
        name            = "parent",
        type            = "STRING",
        is_computed     = true,
        computation_ddl = provider::alis::resource_name_ancestor_ddl("name", "shelves"),
        is_stored       = true,
      },
      {
        name            = "book_id",
        type            = "STRING",
        is_computed     = true,
        computation_ddl = provider::alis::resource_name_id_ddl("name", "books"),
        is_stored       = true,
      },
    ]
  }
}
