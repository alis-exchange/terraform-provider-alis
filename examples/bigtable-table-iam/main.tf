terraform {
  required_providers {
    bigtable = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "alis" {
  host = "localhost:8080"
}

data "alis_bigtable_table_iam_policy" "policy" {
  project  = "mentenova-db-prod-woi"
  instance = "default"
  table    = "mentenova-db-prod-woi-test"
}

#data "google_iam_policy" "admin" {
#  binding {
#    role    = "roles/bigtable.user"
#    members = [
#      "user:jane@example.com",
#    ]
#  }
#  binding {
#    role    = "roles/bigtable.reader"
#    members = []
#  }
#}
#
#resource "alis_bigtable_table_iam_policy" "editor" {
#  project     = "mentenova-db-prod-woi"
#  instance    = "default"
#  table       = "mentenova-db-prod-woi-test"
#  policy_data = data.google_iam_policy.admin.policy_data
#}
#
#resource "alis_bigtable_table_iam_binding" "editor" {
#  project  = "mentenova-db-prod-woi"
#  instance = "default"
#  table    = "mentenova-db-prod-woi-test"
#  role     = "roles/bigtable.user"
#  members  = [
#    "user:jane@example.com",
#  ]
#}
#
#resource "alis_bigtable_table_iam_member" "editor" {
#  project  = "mentenova-db-prod-woi"
#  instance = "default"
#  table    = "mentenova-db-prod-woi-test"
#  role     = "roles/bigtable.user"
#  member   = "user:jane@example.com"
#}

output "test_iam" {
  description = "The IAM policy for the table"
  value = data.alis_bigtable_table_iam_policy.policy
}
