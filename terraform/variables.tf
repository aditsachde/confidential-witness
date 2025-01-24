variable "project_id" {
  description = "The ID of the GCP project where resources will be created."
  type        = string
}

variable "region" {
  description = "The GCP region to deploy resources."
  type        = string
}

variable "bootloader" {
  description = "Ref for bootloader image."
  type        = string
  default     = "ghcr.io/aditsachde/confidential-witness@sha256:b634433ac01a0f43c05bbeb257044b990fee51a3128a5a5310192a5bddc9bc2d"
}
