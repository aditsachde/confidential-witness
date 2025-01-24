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
  source     = "./deployment"
  project_id = var.project_id
  region     = var.region
  bootloader = var.bootloader

  depends_on = [module.services]
}

# Remove all project level roles (particularly owner and editor) using google_project_iam_policy
# except for the minimal set required to function
data "google_iam_policy" "minimal_roles" {
  # The compute engine service account need this roles in order to issue an attestation token 
  binding {
    role    = "roles/confidentialcomputing.workloadUser"
    members = [module.deployment.compute_engine_service_account_member]
  }

  # This role is needed for managed instance groups to function
  binding {
    role    = "roles/compute.instanceGroupManagerServiceAgent"
    members = ["serviceAccount:${data.google_project.project.number}@cloudservices.gserviceaccount.com"]
  }

  binding {
    role    = "roles/cloudkms.publicKeyViewer"
    members = [module.deployment.github_action_iam_member]
  }
  binding {
    role    = "roles/logging.privateLogViewer"
    members = [module.deployment.github_action_iam_member]
  }
  binding {
    role    = "roles/iam.securityReviewer"
    members = [module.deployment.github_action_iam_member]
  }
  binding {
    role    = "roles/iam.workloadIdentityPoolViewer"
    members = [module.deployment.github_action_iam_member]
  }
}

resource "google_project_iam_policy" "minimal_roles" {
  project     = var.project_id
  policy_data = data.google_iam_policy.minimal_roles.policy_data

  depends_on = [module.deployment]
}
