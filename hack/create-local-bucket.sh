#!/bin/bash

bucket_name=label-mutex
project_id=fake-project
endpoint=${1:-fake-gcs-server}

gsutil -o "Credentials:gs_json_host=$endpoint" -o "Credentials:gs_json_port=4443" -o "Boto:https_validate_certificates=False" mb -p "${project_id}" "gs://${bucket_name}"

# apparently implied with fake-gcs-server
# gsutil -o "Credentials:gs_json_host=fake-gcs-server" -o "Credentials:gs_json_port=4443" -o "Boto:https_validate_certificates=False" versioning set on  -p "${project_id}" "gs://${bucket_name}"
