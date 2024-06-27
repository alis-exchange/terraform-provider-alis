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

data "alis_google_discovery_engine_data_store_schemas" "schemas" {
  project       = var.ALIS_OS_PROJECT
  location      = "global"
  collection_id = "default_collection"
  data_store_id = var.ALIS_OS_DISCOVERY_ENGINE_DATASTORE
}


output "test_iam" {
  description = "Available schemas"
  value       = data.alis_google_discovery_engine_data_store_schemas.schemas
}


