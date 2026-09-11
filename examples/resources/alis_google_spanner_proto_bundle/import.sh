# A proto bundle slice is imported by database and package. Sources are not
# recorded in Spanner, so the first plan after import shows an in-place
# update that re-sends the package's descriptors from the configured sources.
terraform import alis_google_spanner_proto_bundle.books "projects/{project}/instances/{instance}/databases/{database}/protoBundles/{package}"
