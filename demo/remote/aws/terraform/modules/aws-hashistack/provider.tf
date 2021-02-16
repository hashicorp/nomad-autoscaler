terraform {
  required_version = ">= 0.12"
}

provider "aws" {
  version = "~> 2.65"
  region  = var.region
}
