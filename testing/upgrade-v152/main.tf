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

resource "alis_google_spanner_table" "test" {
  project  = "test-project"
  instance = "test-instance"
  database = "test-database"
  name     = "tftest_upgrade"

  schema = {
    columns = [
      {
        name           = "id"
        type           = "INT64"
        is_primary_key = true
        auto_increment = true
      },
      {
        name     = "display_name"
        type     = "STRING"
        size     = 255
        required = true
      },
      {
        name   = "email"
        type   = "STRING"
        size   = 255
        unique = true
      },
      {
        name      = "score"
        type      = "FLOAT64"
        precision = 17
        scale     = 2
      },
      {
        name          = "is_active"
        type          = "BOOL"
        default_value = "true"
      },
      {
        name             = "updated_at"
        type             = "TIMESTAMP"
        auto_update_time = true
      },
      {
        name            = "email_upper"
        type            = "STRING"
        size            = 255
        is_computed     = true
        computation_ddl = "UPPER(email)"
      },
    ]
  }
}
