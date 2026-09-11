# fds/fds_including_imports is compiled from internal/spanner/schema/testdata/
# proto_bundle/owned.proto: package tftest.v1 (Simple, Simple.Nested, Kind)
# importing tfdep.Shared. Swap in a real fds_including_imports and package to
# test against a Define output.
resource "alis_google_spanner_proto_bundle" "test_bundle" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  packages = ["tftest.v1"]
  sources = [
    { local_path = "${path.module}/fds/fds_including_imports" },
  ]
}

# A PROTO column proves the dependency both ways: the table cannot be created
# before the bundle, and the bundle cannot drop tftest.v1.Simple while the
# column exists.
resource "alis_google_spanner_table" "test_bundle_table" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  name     = "tf_test_proto_bundle"
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name          = "simple",
        type          = "PROTO",
        proto_package = "tftest.v1.Simple",
      },
    ]
  }

  depends_on = [alis_google_spanner_proto_bundle.test_bundle]
}
