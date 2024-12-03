provider "google" {
  project = var.project_id
  region  = var.region
}

module "services" {
  source     = "./services"
  project_id = var.project_id
}

module "deployment" {
  source     = "./deployment"
  project_id = var.project_id
  region     = var.region
  image_digest = var.image_digest

  depends_on = [module.services]
}

# # Remove all project level roles (particularly owner and editor) using google_project_iam_policy
# resource "google_project_iam_policy" "remove_owner" {
#   project = var.project_id

#   policy_data = jsonencode({
#     bindings = []
#   })

#   depends_on = [module.deployment]
# }
