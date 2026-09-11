data "alis_google_spanner_table_index" "books_by_title" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "books"
  name     = "books_by_title"
}

output "books_by_title_unique" {
  value = data.alis_google_spanner_table_index.books_by_title.unique
}
