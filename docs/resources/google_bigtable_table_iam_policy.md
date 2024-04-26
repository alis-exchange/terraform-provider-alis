---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "alis_google_bigtable_table_iam_policy Resource - alis"
subcategory: ""
description: |-
  
---

# alis_google_bigtable_table_iam_policy (Resource)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `bindings` (Attributes List) (see [below for nested schema](#nestedatt--bindings))
- `instance` (String) The Bigtable instance ID.
- `project` (String) The Google Cloud project ID.
- `table` (String) The Bigtable table ID.

<a id="nestedatt--bindings"></a>
### Nested Schema for `bindings`

Required:

- `members` (List of String)
- `role` (String)