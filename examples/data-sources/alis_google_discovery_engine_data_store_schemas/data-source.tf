data "alis_google_discovery_engine_data_store_schemas" "schemas" {
  project       = var.GOOGLE_PROJECT
  location      = "global"
  collection_id = "default_collection"
  data_store_id = var.DISCOVERY_ENGINE_DATASTORE
}

