data "alis_google_spanner_table_ttl_policy" "events" {
  project  = var.GOOGLE_PROJECT
  instance = var.SPANNER_INSTANCE
  database = "tf-test"
  table    = "events"
}

output "events_ttl_days" {
  value = data.alis_google_spanner_table_ttl_policy.events.ttl
}
