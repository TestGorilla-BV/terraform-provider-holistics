terraform {
  required_providers {
    holistics = {
      source = "TestGorilla-BV/holistics"
    }
  }
}

variable "holistics_api_key" {
  type      = string
  sensitive = true
}

provider "holistics" {
  api_key = var.holistics_api_key
  region  = "apac"
}
