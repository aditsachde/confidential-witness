resource "google_project_service" "compute" {
  project            = var.project_id
  service            = "compute.googleapis.com"
  disable_on_destroy = false
}

# Set default network service tier for compute engine
resource "google_compute_project_default_network_tier" "default" {
  network_tier = "STANDARD"
  depends_on   = [google_project_service.compute]
}

resource "google_project_service" "cloudkms" {
  project            = var.project_id
  service            = "cloudkms.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iam" {
  project            = var.project_id
  service            = "iam.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "confidentialcomputing" {
  project            = var.project_id
  service            = "confidentialcomputing.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloudresourcemanager" {
  project            = var.project_id
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

# Remove all default service accounts. Specifically used for removing the default compute service account
resource "google_project_default_service_accounts" "remove_default" {
  project = var.project_id
  action  = "DEPRIVILEGE"
}
