terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 0.0.1-alpha4"
    }
  }
}

provider "alis" {
}
