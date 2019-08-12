#!/bin/bash

set -eou pipefail

## Setup svc acct
echo $GCE_SVC_ACCT | base64 -d > /tmp/svc-acct.json
apk add --no-cache python
curl -L -o /tmp/google-cloud-sdk.tar.gz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-253.0.0-linux-x86_64.tar.gz
tar -xf /tmp/google-cloud-sdk.tar.gz -C /tmp
/tmp/google-cloud-sdk/install.sh --disable-installation-options --quiet
/tmp/google-cloud-sdk/bin/gcloud auth activate-service-account --key-file /tmp/svc-acct.json

## Push talos-gce to storage bucket
/tmp/google-cloud-sdk/bin/gsutil cp ./build/gce.tar.gz gs://talos-e2e/gce-${TAG}.tar.gz

## Create image from talos-gce
/tmp/google-cloud-sdk/bin/gcloud --quiet --project talos-testbed compute images delete talos-e2e-${TAG} || true ##Ignore error if image doesn't exist
/tmp/google-cloud-sdk/bin/gcloud --quiet --project talos-testbed compute images create talos-e2e-${TAG} --source-uri gs://talos-e2e/gce-${TAG}.tar.gz
