data "alis_google_spanner_table_foreign_key" "orders_book" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "orders"
  name     = "FK_orders_book"
}

output "orders_book_on_delete" {
  value = data.alis_google_spanner_table_foreign_key.orders_book.on_delete
}
