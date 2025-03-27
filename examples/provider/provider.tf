terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 1.5.0, < 2.0.0"
    }
  }
}

provider "alis" {
  project = var.GOOGLE_PROJECT
}