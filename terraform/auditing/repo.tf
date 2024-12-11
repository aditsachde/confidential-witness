terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }
}

locals {
  repo_name = split("/", var.repository)[1]
}

resource "github_repository_file" "digest" {
  repository = local.repo_name
  file       = "digest"
  content    = var.image_digest
}


resource "github_repository_file" "workflow" {
  repository = local.repo_name
  file       = ".github/workflows/verify-and-sign.yml"
  content = templatefile("${path.module}/verify-and-sign.yml", {
    workload_identity_provider = "${google_iam_workload_identity_pool_provider.github_provider.name}",
    crypto_key_reference       = "${google_kms_crypto_key.signing_key.id}/cryptoKeyVersions/1",
  })
}
