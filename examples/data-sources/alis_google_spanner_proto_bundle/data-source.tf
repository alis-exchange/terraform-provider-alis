data "alis_google_spanner_proto_bundle" "books" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  packages = ["com.example.books.v1"]
}

output "books_proto_types" {
  value = data.alis_google_spanner_proto_bundle.books.types
}
