resource "alis_google_spanner_database_sequence" "test_sequence" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  sequence = "test_sequence"
  options = {
    sequence_kind = "bit_reversed_positive"
    skip_range = {
      min = 1000
      max = 5000000
    }
    start_with_counter = 1000
  }
}
