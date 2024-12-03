variable "project_id" {
  description = "The ID of the GCP project where resources will be created."
  type        = string
}

variable "region" {
  description = "The GCP region to deploy resources."
  type        = string
}

variable "image_digest" {
  description = "SHA256 hash of the container to deploy."
  type        = string
}
