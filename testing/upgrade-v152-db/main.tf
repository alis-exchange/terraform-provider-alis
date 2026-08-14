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

# resource "alis_google_spanner_database" "test" was removed from config;
# it still exists in state to test upgrade behavior.
