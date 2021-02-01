variable "org_id" {
  description = "The Google Cloud Platform organization where resources will be created."
  type        = string
}

variable "billing_account" {
  description = "The billing account that will be linked to the project."
  type        = string
}
