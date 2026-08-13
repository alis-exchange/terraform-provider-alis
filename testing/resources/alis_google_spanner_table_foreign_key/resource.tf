// Local manual verification for alis_google_spanner_table_foreign_key.
// Self-contained: creates a parent/child table pair, then the foreign key
// between them, in one apply.
resource "alis_google_spanner_table" "tf_fk_users" {
  project         = var.GOOGLE_PROJECT
  instance        = var.SPANNER_INSTANCE
  database        = var.SPANNER_DATABASE
  name            = "tf_fk_users"
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name = "email",
        type = "STRING",
        size = 255,
      },
    ]
  }
}

resource "alis_google_spanner_table" "tf_fk_orders" {
  project         = var.GOOGLE_PROJECT
  instance        = var.SPANNER_INSTANCE
  database        = var.SPANNER_DATABASE
  name            = "tf_fk_orders"
  prevent_destroy = false
  schema = {
    columns = [
      {
        name           = "id",
        type           = "INT64",
        is_primary_key = true,
        required       = true,
      },
      {
        name     = "user_id",
        type     = "INT64",
        required = true,
      },
      {
        name = "amount",
        type = "FLOAT64",
      },
    ]
  }
}

resource "alis_google_spanner_table_foreign_key" "test_fk" {
  project           = var.GOOGLE_PROJECT
  instance          = var.SPANNER_INSTANCE
  database          = var.SPANNER_DATABASE
  table             = alis_google_spanner_table.tf_fk_orders.name
  name              = "FK_tf_fk_orders_user"
  column            = "user_id"
  referenced_table  = alis_google_spanner_table.tf_fk_users.name
  referenced_column = "id"
  on_delete         = "CASCADE"

  depends_on = [
    alis_google_spanner_table.tf_fk_users,
    alis_google_spanner_table.tf_fk_orders,
  ]
}
