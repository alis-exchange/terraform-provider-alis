terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 2.0.0, < 3.0.0"
    }
  }
}

provider "alis" {
  project = var.GOOGLE_PROJECT
}