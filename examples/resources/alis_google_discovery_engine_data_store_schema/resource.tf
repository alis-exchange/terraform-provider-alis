resource "alis_google_discovery_engine_data_store_schema" "schema" {
  project       = var.GOOGLE_PROJECT
  location      = "global"
  collection_id = "default_collection"
  data_store_id = var.DISCOVERY_ENGINE_DATASTORE
  schema_id     = "default_schema"
  schema = jsonencode({
    "$schema" : "https://json-schema.org/draft/2020-12/schema",
    "type" : "object",
    "date_detection" : true,
    "properties" : {
      "name" : {
        "type" : "string",
        "keyPropertyMapping" : "title",
        "retrievable" : true,
        "dynamicFacetable" : false,
        "completable" : false
      },
      "displayName" : {
        "type" : "string",
        "searchable" : true,
        "retrievable" : true,
        "indexable" : true,
        "dynamicFacetable" : true,
        "completable" : false
      }
    }
  })
}

