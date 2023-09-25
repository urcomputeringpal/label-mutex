#!/usr/bin/env bash

# Copyright 2022 The Sigstore Authors
# Copyright 2023 Ur Computering Pal, LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Idempotent script.
#
# Commands based off of Google blog post
# https://cloud.google.com/blog/products/identity-security/enabling-keyless-authentication-from-github-actions
#
# One addition is the attribute.repository=assertion.repository mapping.
# This allows it to be pinned to given repo.
# https://console.cloud.google.com/flows/enableapi?apiid=iam.googleapis.com,cloudresourcemanager.googleapis.com,iamcredentials.googleapis.com,sts.googleapis.com&redirect=https://console.cloud.google.com

set -o errexit
set -o nounset
set -o pipefail
# set -o verbose
# set -o xtrace

PROJECT_ID="${PROJECT_ID:?Environment variable PROJECT_ID must be set.}"
PROJECT_NUMBER="${PROJECT_NUMBER:?Environment variable PROJECT_NUMBER must be set.}"
REPO="${REPO:?Environment variable REPO must be set.}"
DEFAULT_NAME="${REPO//\//-}-github-actions-label-mutex"
RESOURCE_NAME="${RESOURCE_NAME:-${DEFAULT_NAME}}"

# explain how many characters are in NAME is and complain if its longer than 32
if [ ${#RESOURCE_NAME} -gt 32 ]; then
  echo "RESOURCE_NAME is too long, must be less than 32 characters. Set a shorter one to continue."
  exit 1
fi

BUCKET="${BUCKET:-${RESOURCE_NAME}}"
POOL_NAME="${POOL_NAME:-${RESOURCE_NAME}}"
PROVIDER_NAME="${PROVIDER_NAME:-${RESOURCE_NAME}}"
SERVICE_ACCOUNT_ID="${SERVICE_ACCOUNT_ID:-${RESOURCE_NAME}}"
LOCATION="global"
SERVICE_ACCOUNT="${SERVICE_ACCOUNT_ID}@${PROJECT_ID}.iam.gserviceaccount.com"

for service in \
  cloudresourcemanager.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  sts.googleapis.com \
  ; do
  gcloud services enable $service --project "${PROJECT_ID}"
done

# Setup bucket if it doesn't exist
if ! (gsutil ls "gs://${BUCKET}"); then
  gsutil mb -p "${PROJECT_ID}" "gs://${BUCKET}"
fi
gsutil versioning set on "gs://${BUCKET}"

# Create workload identity pool if not present.
if ! (gcloud iam workload-identity-pools describe "${POOL_NAME}" --location=${LOCATION}); then
  gcloud iam workload-identity-pools create "${POOL_NAME}" \
    --project="${PROJECT_ID}" \
    --location="${LOCATION}"
fi

# Create workload identity provider if not present.
if ! (gcloud iam workload-identity-pools providers describe "${PROVIDER_NAME}" --location="${LOCATION}" --workload-identity-pool="${POOL_NAME}"); then
  gcloud iam workload-identity-pools providers create-oidc "${PROVIDER_NAME}" \
  --project="${PROJECT_ID}" \
  --location="${LOCATION}" \
  --workload-identity-pool="${POOL_NAME}" \
  --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.aud=assertion.aud,attribute.repository=assertion.repository" \
  --issuer-uri="https://token.actions.githubusercontent.com"

  echo "It can take up to 5 minutes from when you configure the Workload Identity Pool mapping until the permissions are available."
fi

# Create service account if not present.
if ! (gcloud iam service-accounts describe "${SERVICE_ACCOUNT}"); then
gcloud iam service-accounts create ${SERVICE_ACCOUNT_ID} \
  --description="Service account for Github Actions ${REPO} label-mutex"
fi

# Adding binding is idempotent.
gcloud iam service-accounts add-iam-policy-binding "${SERVICE_ACCOUNT}" \
  --project="${PROJECT_ID}" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/${LOCATION}/workloadIdentityPools/${POOL_NAME}/attribute.repository/${REPO}"

# Adding binding is idempotent.
gcloud storage buckets add-iam-policy-binding "gs://${BUCKET}" \
  --project="${PROJECT_ID}" \
  --role="roles/storage.objectAdmin" \
  --member="serviceAccount:${SERVICE_ACCOUNT}"

cat <<- EOM
  # Example workflow
  jobs:
    gcloud-authenticated-job:
      runs-on: ubuntu-latest
      permissions:
        contents: read
        id-token: write
      steps:
        - uses: actions/checkout@v4
        - id: auth
          name: Authenticate to Google Cloud
          uses: google-github-actions/auth@v1
          with:
            workload_identity_provider: projects/${PROJECT_NUMBER}/locations/${LOCATION}/workloadIdentityPools/${POOL_NAME}/providers/${PROVIDER_NAME}
            service_account: ${SERVICE_ACCOUNT}
            create_credentials_file: true
            export_environment_variables: true
            access_token_scopes: https://www.googleapis.com/auth/devstorage.full_control
        - uses: docker://ghcr.io/urcomputeringpal/label-mutex:v0.4.0
          id: label-mutex
          with:
            GITHUB_TOKEN: \${{ secrets.GITHUB_TOKEN }}
            bucket: ${BUCKET}
            lock: example-lock
            label: example-lock
EOM

