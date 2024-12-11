terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "github" {}

data "google_project" "project" {
  project_id = var.project_id
}

module "services" {
  source     = "./services"
  project_id = var.project_id
}

module "signing" {
  source       = "./signing"
  project_id   = var.project_id
  region       = var.region
  repository   = var.repository
  image_digest = var.image_digest

  depends_on = [module.services]
}

module "deployment" {
  source          = "./deployment"
  project_id      = var.project_id
  region          = var.region
  key_fingerprint = module.signing.key_fingerprint
  repository      = var.repository

  depends_on = [module.signing]
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

  # These roles are used to retrieve data used for audits
  audit_config {
    service = "cloudkms.googleapis.com"
    audit_log_configs {
      log_type         = "DATA_READ"
      # Exempt due to the high volume of actions performed in the trusted image
      exempted_members = [module.deployment.trusted_image_iam_member]
    }
  }

  binding {
    role    = "roles/cloudkms.publicKeyViewer"
    members = [module.signing.github_action_iam_member]
  }
  binding {
    role    = "roles/logging.privateLogViewer"
    members = [module.signing.github_action_iam_member]
  }
  binding {
    role    = "roles/iam.securityReviewer"
    members = [module.signing.github_action_iam_member]
  }
  binding {
    role    = "roles/iam.workloadIdentityPoolViewer"
    members = [module.signing.github_action_iam_member]
  }
}

resource "google_project_iam_policy" "minimal_roles" {
  project     = var.project_id
  policy_data = data.google_iam_policy.minimal_roles.policy_data

  depends_on = [module.deployment]
}
