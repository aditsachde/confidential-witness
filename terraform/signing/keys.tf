data "google_project" "project" {
  project_id = var.project_id
}

resource "google_kms_key_ring" "signing_keyring" {
  name     = "signing-keyring"
  location = var.region
}

resource "google_iam_workload_identity_pool" "github_provider" {
  workload_identity_pool_id = "github-actions-pool"
}

resource "google_iam_workload_identity_pool_provider" "github_provider" {
  workload_identity_pool_provider_id = "github-provider"
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_provider.workload_identity_pool_id

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"       = "assertion.sub",
    "attribute.actor"      = "assertion.actor"
    "attribute.aud"        = "assertion.aud"
    "attribute.repository" = "assertion.repository"
  }

  attribute_condition = <<EOF
    attribute.repository=='${var.repository}' &&
    assertion.ref=='refs/heads/main' &&
    assertion.ref_type=='branch'
  EOF
}

locals {
  github_action_iam_member = "principalSet://iam.googleapis.com/projects/${
    data.google_project.project.number
    }/locations/global/workloadIdentityPools/${
    google_iam_workload_identity_pool_provider.github_provider.workload_identity_pool_id
    }/attribute.repository/${
    var.repository
  }"
}

data "google_iam_policy" "signing_action" {
  binding {
    role = "roles/cloudkms.viewer"
    members = ["principalSet://iam.googleapis.com/projects/${
      data.google_project.project.number
      }/locations/global/workloadIdentityPools/${
      google_iam_workload_identity_pool_provider.github_provider.workload_identity_pool_id
      }/attribute.repository/${
      var.repository
    }"]
  }

  binding {
    role    = "roles/cloudkms.signerVerifier"
    members = [local.github_action_iam_member]
  }
}

output "github_action_iam_member" {
  value = local.github_action_iam_member
}

resource "google_kms_key_ring_iam_policy" "signing_action_binding" {
  key_ring_id = google_kms_key_ring.signing_keyring.id
  policy_data = data.google_iam_policy.signing_action.policy_data
}

resource "google_kms_crypto_key" "signing_key" {
  name     = "signing-key"
  key_ring = google_kms_key_ring.signing_keyring.id
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm        = "EC_SIGN_P256_SHA256"
    protection_level = "SOFTWARE"
  }

  depends_on = [google_kms_key_ring_iam_policy.signing_action_binding]
}

# Retain _Default logs for 400 days to match admin activity logs
# These are used to log signing key usage
resource "google_logging_project_bucket_config" "retain_logs" {
    project    = var.project_id
    location  = "global"
    retention_days = 400
    bucket_id = "_Default"
}

data "external" "fingerprint" {
  program = ["bash", "${path.module}/fingerprint.sh"]
  query = {
    "key" = "${google_kms_crypto_key.signing_key.id}/cryptoKeyVersions/1"
  }
}

output "key_fingerprint" {
  value = data.external.fingerprint.result["fingerprint"]
}
