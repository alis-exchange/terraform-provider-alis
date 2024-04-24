terraform {
  required_providers {
    google = {
      source = "alis.exchange/db/alis"
    }
  }
  required_version = ">= 1.1.0"
}

provider "google" {
  host = "localhost:8080"
}