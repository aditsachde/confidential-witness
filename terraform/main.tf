provider "google" {
  project = var.project_id
  region  = var.region
}

data "google_project" "project" {
  project_id = var.project_id
}

module "services" {
  source     = "./services"
  project_id = var.project_id
}

module "deployment" {
  source       = "./deployment"
  project_id   = var.project_id
  region       = var.region
  image_digest = var.image_digest

  depends_on = [module.services]
}

# # Remove all default service accounts. Specifically used for removing the default compute service account
# resource "google_project_default_service_accounts" "remove_default" {
#   project    = var.project_id
#   action     = "DELETE"
#   depends_on = [module.services]
# }


# Remove all project level roles (particularly owner and editor) using google_project_iam_policy
# except for the minimal set required to function
data "google_iam_policy" "minimal_roles" {
  # The compute engine service account need this roles in order to issue an attestation token 
  binding {
    role    = "roles/confidentialcomputing.workloadUser"
    members = [module.deployment.compute_engine_service_account_member]
  }

  # These two roles are needed for managed instance groups to function
  binding {
    role    = "roles/compute.instanceAdmin.v1"
    members = ["serviceAccount:${data.google_project.project.number}@cloudservices.gserviceaccount.com"]
  }
  binding {
    role    = "roles/iam.serviceAccountUser"
    members = ["serviceAccount:${data.google_project.project.number}@cloudservices.gserviceaccount.com"]
  }
  binding {
    role    = "roles/owner"
    members = ["user:adit@itko.dev"]
  }
}

resource "google_project_iam_policy" "minimal_roles" {
  project     = var.project_id
  policy_data = data.google_iam_policy.minimal_roles.policy_data

  depends_on = [module.deployment]
}
