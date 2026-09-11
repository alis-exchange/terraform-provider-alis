data "alis_google_spanner_table" "books" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  name     = "books"
}

output "books_columns" {
  value = [for column in data.alis_google_spanner_table.books.schema.columns : column.name]
}
