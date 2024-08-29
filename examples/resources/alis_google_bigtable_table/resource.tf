resource "alis_google_bigtable_table" "test" {
  project  = var.GOOGLE_PROJECT
  instance = var.BIGTABLE_INSTANCE
  name     = "tf-test"
  column_families = [
    {
      name = "0"
    },
  ]
  deletion_protection = false
  #  change_stream_retention = "86400s"
}