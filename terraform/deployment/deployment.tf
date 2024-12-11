data "google_project" "project" {
  project_id = var.project_id
}

resource "google_kms_key_ring" "witness_keyring" {
  name     = "witness-keyring"
  location = var.region
}

resource "google_kms_crypto_key" "witness_key" {
  name     = "witness-key"
  key_ring = google_kms_key_ring.witness_keyring.id
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm        = "EC_SIGN_ED25519"
    protection_level = "SOFTWARE"
  }
}

# ----------------------------------------------------------

resource "google_service_account" "witness_compute_engine" {
  account_id   = "witness-compute-engine"
  display_name = "Service Account used to run the witness on Compute Engine"
}

output "compute_engine_service_account_member" {
  value = google_service_account.witness_compute_engine.member
}

# ----------------------------------------------------------

resource "google_iam_workload_identity_pool" "trusted_workload" {
  workload_identity_pool_id = "trusted-workload-pool"
}

locals {
  witness_key  = "${google_kms_crypto_key.witness_key.id}/cryptoKeyVersions/1"
  witness_name = "confidential-witness-${var.project_id}"
  # The provider name cannot be set automatically because otherwise there is a circular dependency
  witness_audience = "//iam.googleapis.com/${google_iam_workload_identity_pool.trusted_workload.name}/providers/attestation-verifier"
}

resource "google_iam_workload_identity_pool_provider" "attestation_verifier" {
  # Do not change without updating the witness_audience local variable
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
    'STABLE' in assertion.submods.confidential_space.support_attributes &&
    assertion.swname=='CONFIDENTIAL_SPACE' &&
    assertion.submods.gce.project_id=='${var.project_id}' &&
    ['ECDSA_P256_SHA256:${var.key_fingerprint}'].exists(fingerprint, fingerprint in assertion.submods.container.image_signatures.map(sig, sig.signature_algorithm+':'+sig.key_id)) &&
    assertion.submods.container.env.WITNESS_KEY=='${local.witness_key}' &&
    assertion.submods.container.env.WITNESS_NAME=='${local.witness_name}' &&
    assertion.submods.container.env.WITNESS_AUDIENCE=='${local.witness_audience}' &&
    '${google_service_account.witness_compute_engine.email}' in assertion.google_service_accounts
  EOF
}

# ----------------------------------------------------------

locals {
  trusted_image_iam_member = "principalSet://iam.googleapis.com/projects/${
    data.google_project.project.number
    }/locations/global/workloadIdentityPools/${
    google_iam_workload_identity_pool_provider.attestation_verifier.workload_identity_pool_id
  }/*" // already verified by the attribute_condition
}

data "google_iam_policy" "trusted_image" {
  binding {
    role    = "roles/cloudkms.signer"
    members = [local.trusted_image_iam_member]
  }
}

resource "google_kms_key_ring_iam_policy" "trusted_workload_binding" {
  key_ring_id = google_kms_key_ring.witness_keyring.id
  policy_data = data.google_iam_policy.trusted_image.policy_data
}

output "trusted_image_iam_member" {
  value = local.trusted_image_iam_member
}

# ----------------------------------------------------------

resource "google_compute_network" "witness" {
  name = "witness-network"
}

resource "google_compute_firewall" "witness" {
  name    = "witness-firewall"
  network = google_compute_network.witness.name

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["80", "8080"]
  }

  source_ranges = ["0.0.0.0/0"]
}

# ----------------------------------------------------------

resource "google_compute_region_instance_template" "witness_template" {
  name         = "witness-template"
  machine_type = "n2d-highcpu-2"

  metadata = {
    "tee-image-reference"      = "ghcr.io/${var.repository}:latest"
    "tee-signed-image-repos"   = "ghcr.io/${var.repository}"
    "tee-env-WITNESS_KEY"      = local.witness_key
    "tee-env-WITNESS_NAME"     = local.witness_name
    "tee-env-WITNESS_AUDIENCE" = local.witness_audience
  }

  disk {
    source_image = "projects/confidential-space-images/global/images/family/confidential-space"
    boot         = true
    auto_delete  = true
    disk_type    = "pd-standard"
  }

  network_interface {
    access_config {
      network_tier = "PREMIUM"
    }

    network = google_compute_network.witness.self_link
  }

  service_account {
    email  = google_service_account.witness_compute_engine.email
    scopes = ["cloud-platform"]
  }

  scheduling {
    # Required to set provisioning mode to SPOT
    preemptible        = true
    automatic_restart  = false
    provisioning_model = "SPOT"

    # required to prevent recreation on every run
    instance_termination_action = "STOP"

    # Required to set type to SEV_SNP
    on_host_maintenance = "TERMINATE"
  }

  # Required to set type to SEV_SNP
  # Not setting this allows for a larger pool of resources for spot instances to draw from
  # min_cpu_platform = "AMD Milan"

  # Currently, it seems creating SEV_SNP instances results in out of capacity errors
  confidential_instance_config {
    confidential_instance_type = "SEV"
  }

  shielded_instance_config {
    enable_secure_boot = true
  }
}

resource "google_compute_health_check" "witness_running" {
  name                = "witness-running-check"
  check_interval_sec  = 5
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 3 # 15 seconds

  # the witness serves the public key on port 8080 and is always expected to return 200
  http_health_check {
    request_path = "/"
    port         = "8080"
  }
}

resource "google_compute_region_instance_group_manager" "witness_mig" {
  name = "confidential-witness-spot-group"
  # short due to https://issuetracker.google.com/issues/264362370
  base_instance_name = "wit"
  region             = var.region
  target_size        = 1

  version {
    instance_template = google_compute_region_instance_template.witness_template.self_link
  }

  auto_healing_policies {
    health_check      = google_compute_health_check.witness_running.id
    initial_delay_sec = 300
  }

  instance_lifecycle_policy {
    force_update_on_repair = "YES"
  }

  update_policy {
    type                         = "PROACTIVE"
    minimal_action               = "REPLACE"
    max_unavailable_fixed        = 3
    instance_redistribution_type = "NONE"
    replacement_method           = "RECREATE"
    max_surge_fixed = 0
  }

  stateful_external_ip {
    interface_name = "nic0"
    delete_rule    = "ON_PERMANENT_INSTANCE_DELETION"
  }

  lifecycle {
    replace_triggered_by = [google_compute_region_instance_template.witness_template.id]
  }
}
