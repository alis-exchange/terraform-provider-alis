data "alis_google_spanner_database_sequence" "book_ids" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  sequence = "book_ids"
}

output "book_ids_kind" {
  value = data.alis_google_spanner_database_sequence.book_ids.options.sequence_kind
}
