provider "google" {
  region = var.region
}

terraform {
  required_providers {
    google = {
      version = "=3.54.0"
    }
  }
}
