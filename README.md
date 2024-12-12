```
confidential-witness-urban-octo-telegram-us-east5+7395fc96+AU5kaDj7Jcujhmka2FcoB9tJVxCCzfEKGmDRFnrmEPqu
curl urban-octo-telegram.itko.dev/witness/v0/logs
```

# How this works

The private key of a witness needs to be protected from misuse. Armored witness does this by using a microcontroller and its related security features. This project attempts to provide similar guarantees by using GCP's confidential computing support.

The theory of operation is as follows: the following principal set recieves signing key usage permissions (terraform placeholder syntax is used here)

```
"principalSet://iam.googleapis.com/projects/${
  data.google_project.project.number
  }/locations/global/workloadIdentityPools/${
  google_iam_workload_identity_pool_provider.attestation_verifier.workload_identity_pool_id
}/*"
```

The workload identity pool has the issuer ID `https://confidentialcomputing.googleapis.com/` with the following attribute conditions

```
    'STABLE' in assertion.submods.confidential_space.support_attributes &&
    assertion.swname=='CONFIDENTIAL_SPACE' &&
    assertion.submods.gce.project_id=='${var.project_id}' &&
    assertion.submods.container.image_digest=='${var.image_digest}' &&
    assertion.submods.container.env.WITNESS_KEY=='${local.witness_key}' &&
    assertion.submods.container.env.WITNESS_NAME=='${local.witness_name}' &&
    assertion.submods.container.env.WITNESS_AUDIENCE=='${local.witness_audience}' &&
    '${google_service_account.witness_compute_engine.email}' in assertion.google_service_accounts
```

These combination of permissions result in GCP enforcing that the container that is trying to use the key matches a specific digest. This digest is expected to be of a signed image in this repo, allowing it to be traced back to a specific github actions run and source code input.

The usage of a stable, production, confidential space means that GCP prevents the operator of the witness to tamper with it at runtime, preventing them from getting SSH access and pretending to be a trusted image.

To audit this, the outputs of the three following commands are needed: `gcloud iam workload-identity-pools providers describe`, `gcloud kms keys get-iam-policy`, and `gcloud logging read "logName=projects/${project}/logs/cloudaudit.googleapis.com%2Factivity"`.

The first two output the current configuration of the IAM setup to verify that it matches the one described above. The third outputs the admin activity logs. The preferred setup is a Github Actions cron job which runs these three commands every X hours and uploads them as artifacts. This allows for comparing the current activity logs and the previous one to ensure that no configuration changes to the IAM policies were made in the intermediate interval.

## Software updates

### Option 1

Are software updates actually necessary? Is it ok to just replace the witness+private key altogether when a new version of omniwitness needs to be deployed?

### Option 2

What is currently implemented is an [intermediate repository](https://github.com/aditsachde/urban-octo-telegram) which has an image with a `latest` tag. The GH action fetches an image with the specified digest, checks if it was signed by an actions run in this repository, and retags it and resigns it with a mutable tag and a long living private key. The confidential space runtime verifies this signature via the `assertion.submods.container.image_signatures` field, which is used as part of the attribute conditions in place of the image digest.

I think some games can be played by the owner of the repository, which means that the signing key usage needs to be audited through data access logs published by KMS. Not sure I like this solution, but the confidential computing service in GCP is restrictive of what attestation attributes it provides.

### Option 3

The actual container set in the confidential space could be a "bootloader" container which fetches the latest release of the software from this repository. This bootloader container would never change over the life of the witness. If an issue is found in this component, then the witness would be distrusted. 

This option provides a lot more flexibility in the release verification layer, allowing us to define the logic and signature verification. However, this would require careful understanding of the bootloader logic, ensuring that it will not do anything unexpected and is not vurnerable to MTIM style attacks, as the operator would be able to inspect and mess with traffic going in and out of the container.


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