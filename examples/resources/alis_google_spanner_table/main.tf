terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "0.0.2-alpha8"
    }
  }
}

provider "alis" {

}

#resource "alis_google_spanner_table" "test" {
#  project  = var.ALIS_OS_PROJECT
#  instance = var.ALIS_OS_SPANNER_INSTANCE
#  database = "tf-test"
#  name     = "tftest"
#  schema = {
#    columns = [
#      {
#        name           = "id",
#        type           = "INT64",
#        is_primary_key = true,
#        unique         = true,
#        required       = true,
#      },
#      {
#        name = "display_name",
#        type = "STRING",
#        size = 255,
#      },
#      {
#        name = "is_active",
#        type = "BOOL",
#      },
#      {
#        name          = "latest_return",
#        type          = "FLOAT64",
#        default_value = 0.0,
#      },
#      {
#        name          = "earliest_return",
#        type          = "FLOAT64",
#        default_value = 0.0,
#      },
#      {
#        name = "inception_date",
#        type = "DATE",
#      },
#      {
#        name = "last_refreshed_at",
#        type = "TIMESTAMP",
#      },
#      {
#        name = "metadata",
#        type = "JSON",
#      },
#      {
#        name = "data",
#        type = "BYTES",
#      }
#    ],
#    indices = [
#      {
#        name = "display_name_idx",
#        columns = [
#          {
#            name  = "display_name",
#            order = "asc",
#          },
#        ],
#        unique = false,
#      },
#    ]
#  }
#}
#
#output "test_table" {
#  value = alis_google_spanner_table.test
#}

resource "alis_google_spanner_database" "database" {
  project                  = var.ALIS_OS_PROJECT
  instance                 = var.ALIS_OS_SPANNER_INSTANCE
  name                     = "${var.ALIS_OS_PROJECT}_port"
  dialect                  = "POSTGRESQL"
  enable_drop_protection   = true
  version_retention_period = "3600s"
}

resource "alis_google_spanner_table" "hc_table" {
  project  = var.ALIS_OS_PROJECT
  instance = var.ALIS_OS_SPANNER_INSTANCE
  database = alis_google_spanner_database.database.name
  name     = "holdings_commits_positions"
  schema = {
    columns = [
      {
        name           = "branch",
        type           = "STRING",
        is_primary_key = true,
        required       = true,
      },
      {
        name           = "effective_date",
        type           = "DATE",
        is_primary_key = true,
        required       = true,
      },
      {
        name           = "instrument",
        type           = "STRING",
        is_primary_key = true,
        required       = true,
      },
      {
        name = "position_data",
        type = "BYTES",
      }
    ],
    indices = [
      {
        name = "branch_date_instrument_idx",
        columns = [
          {
            name  = "branch",
            order = "asc",
          },
          {
            name  = "effective_date",
            order = "desc",
          },
          {
            name  = "instrument",
            order = "asc",
          },
        ],
        unique = true,
      },
    ]
  }
}
