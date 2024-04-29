terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "0.0.1-alpha8"
    }
  }
}

provider "alis" {

}

resource "alis_google_bigtable_table_iam_member" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_BIGTABLE_INSTANCE
  table    = "tf-test"
  role     = "roles/bigtable.user"
  member   = "serviceAccount:${var.ALIS_OS_SERVICE_ACCOUNT}"
}

output "test_iam" {
  description = "The IAM policy for the table"
  value       = alis_google_bigtable_table_iam_member.editor
}
