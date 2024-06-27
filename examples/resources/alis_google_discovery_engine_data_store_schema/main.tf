terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = "1.1.0"
    }
  }
}

provider "alis" {
}

import {
  id = "projects/${var.ALIS_OS_PROJECT}/locations/global/collections/default_collection/dataStores/${var.ALIS_OS_DISCOVERY_ENGINE_DATASTORE}/schemas/default_schema"
  to = alis_google_discovery_engine_data_store_schema.schema
}

resource "alis_google_discovery_engine_data_store_schema" "schema" {
  project       = var.ALIS_OS_PROJECT
  location      = "global"
  collection_id = "default_collection"
  data_store_id = var.ALIS_OS_DISCOVERY_ENGINE_DATASTORE
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

