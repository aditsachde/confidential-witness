```
ConfidentialWitness-buoyant-planet+af4c4125+AaUzzyskbukNz6DJHmPv7DZ4tG4heL51U2ER21TQN9Zm
curl buoyant-planet.itko.dev/witness/v0/logs
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

The actual container set in the confidential space is a "bootloader" container which fetches the latest release of the software from this repository. This bootloader container never changes over the life of the witness. If an issue is found in this component, then the witness should be distrusted. 

The bootloader code can be found in the `bootloader` directory. One release of the bootloader has been currently made. The built container can be found at `ghcr.io/aditsachde/confidential-witness@sha256:b634433ac01a0f43c05bbeb257044b990fee51a3128a5a5310192a5bddc9bc2d`. The build is [signed with cosign](https://search.sigstore.dev/?logIndex=156590257).

# Deploying

1. Ensuring a default billing configuration exists.
2. Create a new project. Don't touch it.
3. Login
    - `gcloud auth application-default login`
4. Fill out terraform.tfvars
5. Run `terraform apply`
6. Done!

This operation "seals" the project by removing your project owner role, which means that you will no longer be able to make any modifications or access any details of anything in the project. To unseal the project, someone with the Organization Administrator can grant access to the project. The OA cannot do anything beyond granting IAM access to the project, and the fact that IAM access was granted will show up in audit logs.

# TODO

1. Auditing scripts and public audit logs.
2. Fetch checkpoints on startup from distributor and verify that they are signed by other witnesses instead of pure TOFU.

With spot instances, running a witness is $10.58 per month, or currently a bit under $9 in us-east5.
Running a witness on a normal instance costs $44.96 per month.