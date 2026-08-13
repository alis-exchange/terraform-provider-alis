terraform {
  required_providers {
    alis = {
      source = "alis-exchange/alis"
    }
  }
}

provider "alis" {
  project = var.GOOGLE_PROJECT
}
