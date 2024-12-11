#!/bin/bash

# Exit if any of the intermediate steps fail
set -e

# Extract "key" argument from the input into
# KEY shell variables.
# jq will ensure that the values are properly quoted
# and escaped for consumption by the shell.
eval "$(jq -r '@sh "KEY=\(.key)"')"

# Placeholder for whatever data-fetching logic your script implements
PUBLIC_KEY=$(gcloud kms keys versions get-public-key $KEY)
FINGERPRINT=$(echo "$PUBLIC_KEY" | openssl pkey -pubin -inform PEM -outform DER | openssl sha256 | cut -d' ' -f2)

# Safely produce a JSON object containing the result value.
# jq will ensure that the value is properly quoted
# and escaped to produce a valid JSON string.
jq -n --arg fingerprint "$FINGERPRINT" '{"fingerprint":$fingerprint}'