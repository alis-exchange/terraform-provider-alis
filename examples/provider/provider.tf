terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 1.4.0"
    }
  }
}

provider "alis" {
  project = var.GOOGLE_PROJECT
}