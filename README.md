1. Write a witness that derives a ed25519 key from a seed using the key stored in KMS
    - omniwitness bakes in the use of in memory keys so modifying this assumption is hard
2. Build container and repository to deploy omniwitness in a confidential space
3. IAM policies to restrict key usage
4. terraform scripts to make deploying this possible
5. auditing scripts
6. gcs backend for persistence to prevent TOFU on instance preemption
    - an alternative is litestream
7. Modify omniwitness to allow using ed25519 keys in KMS

with spot instances, this is $10.58 per month
for norm  instances, this is $44.96 per month

figure out how to have a fixed external ip
setup github repo that cosigns image with a fixed key
figure out how to have the application ask gcp to recreate the instance 
    in order to update the underlying disk image as well as the container image