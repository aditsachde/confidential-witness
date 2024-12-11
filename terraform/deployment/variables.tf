variable "project_id" {
  description = "The ID of the GCP project where resources will be created."
  type        = string
}

variable "region" {
  description = "The GCP region to deploy resources."
  type        = string
}

variable "key_fingerprint" {
  description = "Fingerprint of the image signing key."
  type        = string
}

variable "repository" {
  description = "GitHub repository for auditing workflows."
  type        = string
}
