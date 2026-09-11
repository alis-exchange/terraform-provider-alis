# One package's slice of the database's proto bundle. Define writes the
# descriptor set as fds_including_imports next to the neuron's infra folder;
# a gs:// object or prefix works the same way.
resource "alis_google_spanner_proto_bundle" "books" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  packages = ["com.example.books.v1"]
  sources = [
    { local_path = "${path.module}/../fds_including_imports" },
    # { gcs_uri = "gs://my-bucket/com.example.books.v1/fds_including_imports" },
  ]
}

# Tables with PROTO columns must wait for the bundle: Spanner rejects a column
# whose type is not in the bundle yet, and refuses to delete a type a column
# still references.
resource "alis_google_spanner_table" "books" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  name     = "books"
  schema = {
    columns = [
      {
        name           = "id",
        type           = "STRING",
        size           = 36,
        is_primary_key = true,
        required       = true,
      },
      {
        name          = "book",
        type          = "PROTO",
        proto_package = "com.example.books.v1.Book",
      },
    ]
  }

  depends_on = [alis_google_spanner_proto_bundle.books]
}
