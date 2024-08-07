---
page_title: "alis_google_spanner_table_iam_binding Resource - alis"
subcategory: ""
description: |-
  Authoritative for a given role. Updates the table IAM policy to grant a role along with permissions.
  Other roles and permissions within the IAM policy for the table are preserved.
---

# alis_google_spanner_table_iam_binding (Resource)

Authoritative for a given role. Updates the table IAM policy to grant a role along with permissions.
Other roles and permissions within the IAM policy for the table are preserved.

## Example Usage

```terraform
terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "1.1.2"
    }
  }
}

provider "alis" {

}

resource "alis_google_spanner_table_iam_binding" "editor" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = "tf-test"
  table    = "tftest"
  role     = "admin"
  permissions = [
    "SELECT",
    "UPDATE",
    "INSERT",
    "DELETE",
  ]
}

output "test_iam" {
  description = "The IAM policy for the database"
  value       = alis_google_spanner_table_iam_binding.editor
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `database` (String)
- `instance` (String)
- `permissions` (List of String) The permissions that should be granted to the role.
Valid permissions are: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.
- `project` (String)
- `role` (String) The role that should be granted to the table.
- `table` (String)