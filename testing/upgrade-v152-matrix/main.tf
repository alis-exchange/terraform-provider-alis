// Full v1.5.2 option matrix: every column type and column/resource option the
// v1 provider accepted, applied with registry v1.5.2, then planned with the
// local v2 build to catalog upgrade behavior. Not committed CI material —
// a hands-on upgrade lab.
terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "1.5.2"
    }
  }
}

provider "alis" {
  project = "test-project"
}

locals {
  project  = "test-project"
  instance = "test-instance"
  database = "matrix-db"
}

resource "alis_google_spanner_table" "parent" {
  project  = local.project
  instance = local.instance
  database = local.database
  name     = "tftest_parent"

  schema = {
    columns = [
      {
        name           = "id"
        type           = "INT64"
        is_primary_key = true
        default_value  = "GET_NEXT_SEQUENCE_VALUE(SEQUENCE tftest_parent_seq)"
      },
      {
        name     = "str_sized"
        type     = "STRING"
        size     = 255
        required = true
      },
      {
        name = "str_max"
        type = "STRING"
      },
      {
        name = "str_unique"
        type = "STRING"
        size = 64
      },
      {
        name = "num_float"
        type = "FLOAT64"
      },
      {
        name          = "flag_default"
        type          = "BOOL"
        default_value = "true"
      },
      {
        name          = "int_default"
        type          = "INT64"
        default_value = "10"
      },
      {
        name          = "str_default"
        type          = "STRING"
        size          = 50
        default_value = "'hello'"
      },
      {
        name          = "ts_default"
        type          = "TIMESTAMP"
        default_value = "CURRENT_TIMESTAMP()"
      },
      {
        name = "created_at"
        type = "TIMESTAMP"
      },
      {
        name             = "updated_at"
        type             = "TIMESTAMP"
        auto_update_time = true
      },
      {
        name = "birth_date"
        type = "DATE"
      },
      {
        name = "payload"
        type = "BYTES"
        size = 1024
      },
      {
        name = "meta"
        type = "JSON"
      },
      {
        name = "tags"
        type = "ARRAY<STRING>"
      },
      {
        name = "scores"
        type = "ARRAY<INT64>"
      },
      {
        name = "vec32"
        type = "ARRAY<FLOAT32>"
      },
      {
        name = "vec64"
        type = "ARRAY<FLOAT64>"
      },
      {
        name            = "full_name"
        type            = "STRING"
        size            = 512
        is_computed     = true
        computation_ddl = "CONCAT(str_sized, ' ', COALESCE(str_max, ''))"
      },
      {
        name            = "computed_int"
        type            = "INT64"
        is_computed     = true
        computation_ddl = "id * 2"
      },
    ]
  }
}

resource "alis_google_spanner_table" "child" {
  project  = local.project
  instance = local.instance
  database = local.database
  name     = "tftest_child"

  schema = {
    columns = [
      {
        name           = "id"
        type           = "INT64"
        is_primary_key = true
      },
      {
        name     = "parent_id"
        type     = "INT64"
        required = true
      },
      {
        name = "note"
        type = "STRING"
        size = 100
      },
    ]
  }
}

resource "alis_google_spanner_table_index" "unique_desc" {
  project  = local.project
  instance = local.instance
  database = local.database
  table    = alis_google_spanner_table.parent.name
  name     = "tftest_parent_by_str_unique"
  unique   = true
  columns = [
    {
      name  = "str_unique"
      order = "desc"
    },
  ]
}

resource "alis_google_spanner_table_index" "composite" {
  project  = local.project
  instance = local.instance
  database = local.database
  table    = alis_google_spanner_table.parent.name
  name     = "tftest_parent_by_date_flag"
  columns = [
    {
      name  = "birth_date"
      order = "asc"
    },
    {
      name = "created_at"
    },
  ]
}

resource "alis_google_spanner_table_foreign_key" "child_parent" {
  project           = local.project
  instance          = local.instance
  database          = local.database
  table             = alis_google_spanner_table.child.name
  name              = "fk_tftest_child_parent"
  column            = "parent_id"
  referenced_table  = alis_google_spanner_table.parent.name
  referenced_column = "id"
  on_delete         = "CASCADE"
}

resource "alis_google_spanner_table_ttl_policy" "parent_ttl" {
  project  = local.project
  instance = local.instance
  database = local.database
  table    = alis_google_spanner_table.parent.name
  column   = "created_at"
  ttl      = 30
}

# Dropped from the matrix: v1.5.2 reads roles via an admin RPC the
# emulator does not implement (Unimplemented). Schemas are unchanged
# pass-throughs in v2 and covered by the v2 emulator acceptance tests.
# resource "alis_google_spanner_database_role" "role" {
#   project  = local.project
#   instance = local.instance
#   database = local.database
#   role     = "tftest_role"
# }
#
# resource "alis_google_spanner_table_iam_binding" "binding" {
#   project     = local.project
#   instance    = local.instance
#   database    = local.database
#   table       = alis_google_spanner_table.parent.name
#   role        = alis_google_spanner_database_role.role.role
#   permissions = ["SELECT", "INSERT"]
# }

resource "alis_google_spanner_database_sequence" "parent_seq" {
  project  = local.project
  instance = local.instance
  database = local.database
  sequence = "tftest_parent_seq"
  options = {
    sequence_kind = "bit_reversed_positive"
  }
}
