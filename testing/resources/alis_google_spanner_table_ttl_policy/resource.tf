// Local manual verification for alis_google_spanner_table_ttl_policy.
// Prerequisite: apply testing/resources/alis_google_spanner_table first —
// the policy attaches to tf_test's TIMESTAMP column last_refreshed_at.
resource "alis_google_spanner_table_ttl_policy" "test_ttl" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = var.SPANNER_DATABASE
  table    = var.SPANNER_TABLE
  column   = "last_refreshed_at"
  ttl      = 30
}
