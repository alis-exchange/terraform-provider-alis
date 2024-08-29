resource "alis_google_bigtable_gc_policy" "simple" {
  project       = var.GOOGLE_PROJECT
  instance      = var.BIGTABLE_INSTANCE
  table         = "tf-test"
  column_family = "0"
  gc_rules = jsonencode({
    "rules" : [
      {
        "max_version" : 10
      }
    ]
  })
}

resource "alis_google_bigtable_gc_policy" "complex_union" {
  project       = var.GOOGLE_PROJECT
  instance      = var.BIGTABLE_INSTANCE
  table         = "tf-test"
  column_family = "1"
  gc_rules = jsonencode({
    mode = "union",
    rules = [
      {
        max_age = "168h"
      },
      {
        max_version = 10
      }
    ]
  })
}

resource "alis_google_bigtable_gc_policy" "complex_intersection" {
  project       = var.GOOGLE_PROJECT
  instance      = var.BIGTABLE_INSTANCE
  table         = "tf-test"
  column_family = "2"
  gc_rules = jsonencode({
    mode = "intersection",
    rules = [
      {
        max_age = "168h"
      },
      {
        max_version = 10
      }
    ]
  })
}
