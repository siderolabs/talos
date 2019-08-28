#!/bin/bash

set -eou pipefail

## Setup svc acct
echo $GCE_SVC_ACCT | base64 -d > /tmp/svc-acct.json

gcloud auth activate-service-account --key-file /tmp/svc-acct.json

## Push talos-gcp to storage bucket
gsutil cp ./build/gcp.tar.gz gs://talos-e2e/gcp-${TAG}.tar.gz

## Create image from talos-gcp
gcloud --quiet --project talos-testbed compute images delete talos-e2e-${TAG} || true ##Ignore error if image doesn't exist
gcloud --quiet --project talos-testbed compute images create talos-e2e-${TAG} --source-uri gs://talos-e2e/gcp-${TAG}.tar.gz
