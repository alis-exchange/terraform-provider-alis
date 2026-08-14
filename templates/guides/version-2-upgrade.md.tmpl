---
page_title: "Upgrading to v2.0.0"
subcategory: ""
description: |-
  How to migrate configurations and state from provider v1.x to v2.0.0.
---

# Upgrading to v2.0.0

Version 2.0.0 narrows the provider to Google Cloud Spanner schema management:
tables, indexes, foreign keys, TTL policies, sequences, IAM bindings and
database roles. Everything else that v1.x managed was removed.

State written by any v1.x release (and by the 2.0.0 betas) upgrades
automatically the first time you run `terraform plan` or `terraform apply`
with v2 — every resource now declares schema version 2 and ships a state
upgrader. The first plan after upgrading emits warnings for anything that
needs a config change; read them before applying. The manual work is listed
below.

## Removed resources and data sources

The following are gone. Terraform fails with `Invalid resource type` while a
removed resource is in your configuration, and with
`no schema available for ... while reading state` while one is still in
state — so both the config and the state entry must go.

Removed resources:

* `alis_google_bigtable_table`, `alis_google_bigtable_gc_policy`,
  `alis_google_bigtable_table_iam_policy`,
  `alis_google_bigtable_table_iam_binding`,
  `alis_google_bigtable_table_iam_member`
* `alis_google_spanner_database`, `alis_google_spanner_database_iam_policy`,
  `alis_google_spanner_database_iam_binding`,
  `alis_google_spanner_database_iam_member`
* `alis_google_discovery_engine_data_store_schema`

Removed data sources: `alis_google_bigtable_table_iam_policy`,
`alis_google_spanner_database_iam_policy`,
`alis_google_discovery_engine_data_store_schemas`.

For each one, while still on v1.x (or before the first v2 plan):

1. Remove the block from your configuration.
2. Drop it from state: `terraform state rm alis_google_spanner_database.main`
3. Recreate it under the official Google provider (`google_spanner_database`,
   `google_bigtable_*`, ...) and `terraform import` the existing
   infrastructure there. The underlying Google Cloud resources are untouched
   by `state rm`.

## `alis_google_spanner_table`: removed column attributes

The column attributes `auto_increment`, `unique`, `precision`, `scale` and
`file_descriptor` no longer exist. Terraform silently ignores unknown
attributes inside `schema.columns` objects, so a stale config may still plan
— remove them deliberately.

### `auto_increment` — action required

v1.x implemented `auto_increment = true` by creating a bit-reversed sequence
named `<table>_seq` and a `DEFAULT (GET_NEXT_SEQUENCE_VALUE(...))` on the
column. That sequence still exists in your database, but nothing in a v2
config expresses it — **applying without the replacement below removes the
default from the column**, and inserts that rely on generated IDs stop
working. The first v2 plan warns per affected column.

Replace it with an explicit sequence and column default:

```terraform
resource "alis_google_spanner_database_sequence" "orders_seq" {
  project  = var.project
  instance = var.instance
  database = var.database
  sequence = "orders_seq" # v1.x named it <table>_seq
  options = {
    sequence_kind = "bit_reversed_positive" # what v1.x created
  }
}

resource "alis_google_spanner_table" "orders" {
  # ...
  schema = {
    columns = [
      {
        name           = "id"
        type           = "INT64"
        is_primary_key = true
        default_value  = "GET_NEXT_SEQUENCE_VALUE(SEQUENCE orders_seq)"
      },
      # ...
    ]
  }
}
```

Adopt the sequence v1.x already created instead of making a new one:

```shell
terraform import alis_google_spanner_database_sequence.orders_seq \
  "projects/{project}/instances/{instance}/databases/{database}/sequences/orders_seq"
```

### `unique`

Manage unique indexes explicitly:

```terraform
resource "alis_google_spanner_table_index" "users_email" {
  # ...
  unique  = true
  columns = [{ name = "email" }]
}
```

### `precision`, `scale`

These never affected the stored `FLOAT64` DDL; delete them from the config.

### `file_descriptor`

The provider no longer uploads proto file descriptors. A `PROTO` column is
declared through `proto_package` alone, and the proto bundle must already
exist in the database.

## `default_value` semantics — quote string literals

`default_value` is now the raw expression emitted inside Spanner's
`DEFAULT (...)`; expressions such as `GENERATE_UUID()` or
`GET_NEXT_SEQUENCE_VALUE(SEQUENCE my_seq)` are now supported.

**Action required for `STRING`/`BYTES` defaults**: v1.x quoted string
literals for you (`default_value = "hello"` became `DEFAULT ('hello')`).
v2 emits the value verbatim, so an unquoted literal fails DDL parsing on
the next apply. Wrap it in single quotes:

```terraform
default_value = "'hello'"
```

Numeric, boolean and expression defaults (`10`, `true`,
`CURRENT_TIMESTAMP()`) need no change. The first v2 plan warns per affected
column.

## Computed columns, index `order` and `unique`: no action needed

v1.x always created computed columns as `STORED` with no way to say so in
configuration, and always hydrated index column `order` ("asc") and index
`unique` (false) into state even when the configuration omitted them. In v2
these attributes are Computed: when omitted, they inherit the value already
in the database, so upgraded configurations plan cleanly instead of forcing
a table or index replace. Set them explicitly only when you want to change
them — an explicit change still recreates the table or index, as those
properties cannot be altered in place.

## Leftovers from v1.x

v1.x maintained a `column_metadata` table inside each managed database to
track column settings. v2 neither reads nor writes it; once no v1.x-managed
tables remain, it can be dropped:

```sql
DROP TABLE column_metadata;
```
