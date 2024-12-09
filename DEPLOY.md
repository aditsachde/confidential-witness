Create a new GCP project

```
export PROJECT=mineral-oarlock-443418-j9
```

1. Create a new KMS keyring
2. Attach deny all IAM policies to the keyring
3. Create confidential computing identity pool
4. Create a new key and key verison
5. Fill in key and key version values into the templates
6. create new github repository with said template
7. use sha hash of built container to apply allow IAM policy to keyring
8. use sha hash to create a new compute engine mig that has a confidential spot instance