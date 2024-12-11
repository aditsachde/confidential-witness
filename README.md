# Deploying

1. Ensuring a default billing configuration exists.
2. Create a new project. Don't touch it.
3. Create a new github repository.
4. Login
    - `gcloud auth application-default login`
    - `export GITHUB_TOKEN=`
        - token needs R/W access to `code` and `workflows` on your Github repo
5. Fill out terraform.tfvars
6. Run `terraform apply`
7. Done!

# TODO
1. auditing scripts
2. gcs backend for persistence to prevent TOFU on instance preemption
    - an alternative is litestream
3. Modify omniwitness to allow using ed25519 keys in KMS

with spot instances, this is $10.58 per month
for normal  instances, this is $44.96 per month