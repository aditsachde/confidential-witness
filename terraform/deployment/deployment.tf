data "google_project" "project" {
  project_id = var.project_id
}

resource "google_kms_key_ring" "witness_keyring" {
  name     = "witness-keyring3"
  location = var.region
}

resource "google_kms_crypto_key" "witness_key" {
  name     = "witness-key3"
  key_ring = google_kms_key_ring.witness_keyring.id
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm        = "EC_SIGN_ED25519"
    protection_level = "SOFTWARE"
  }
}

// ----------------------------------------------------------

resource "google_iam_workload_identity_pool" "trusted_workload" {
  workload_identity_pool_id = "trusted-workload-pool"
}

locals {
  witness_key      = "test"
  witness_name     = "test"
  witness_audience = "test"
}

resource "google_iam_workload_identity_pool_provider" "attestation_verifier" {
  workload_identity_pool_provider_id = "attestation-verifier"
  workload_identity_pool_id          = google_iam_workload_identity_pool.trusted_workload.workload_identity_pool_id

  oidc {
    allowed_audiences = ["https://sts.googleapis.com"]
    issuer_uri        = "https://confidentialcomputing.googleapis.com/"
  }

  attribute_mapping = {
    "google.subject"         = "assertion.sub",
    "attribute.image_digest" = "assertion.submods.container.image_digest"
  }

  attribute_condition = <<EOF
    assertion.swname=='CONFIDENTIAL_SPACE' &&
    assertion.submods.gce.project_id=='${var.project_id}' &&
    assertion.submods.container.image_digest=='sha256:${var.image_digest}' &&
    assertion.submods.container.env == {"WITNESS_KEY": "${witness_key}", "WITNESS_NAME": "${witness_name}", "WITNESS_AUDIENCE": "${witness_audience}"}
  EOF
  # "STABLE" in assertion.submods.confidential_space.support_attributes &&
  # 'operator-svc-account@${var.operator_project_id}.iam.gserviceaccount.com' in assertion.google_service_accounts
}

// ----------------------------------------------------------

data "google_iam_policy" "trusted_image" {
  binding {
    role = "roles/cloudkms.signer"
    members = ["principalSet://iam.googleapis.com/projects/${
      data.google_project.project.number
      }/locations/global/workloadIdentityPools/${
      google_iam_workload_identity_pool_provider.attestation_verifier.workload_identity_pool_id
      }/attribute.image_digest/sha256:${
      var.image_digest
    }"]
  }
}

resource "google_kms_key_ring_iam_policy" "trusted_workload_binding" {
  key_ring_id = google_kms_key_ring.witness_keyring.id
  policy_data = data.google_iam_policy.trusted_image.policy_data
}
