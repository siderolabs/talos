#!/bin/bash

set -eou pipefail

## Update secret with service acct info

## Setup svc acct
echo $GCE_SVC_ACCT | base64 -d > /tmp/svc-acct.json
apk add --no-cache python
wget https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-253.0.0-linux-x86_64.tar.gz
tar -xf google-cloud-sdk-253.0.0-linux-x86_64.tar.gz 
./google-cloud-sdk/install.sh --disable-installation-options --quiet
./google-cloud-sdk/bin/gcloud auth activate-service-account --key-file /tmp/svc-acct.json

## Push talos-gce to storage bucket
./google-cloud-sdk/bin/gsutil cp ./build/talos-gce.tar.gz gs://talos-e2e

## Create image from talos-gce
./google-cloud-sdk/bin/gcloud --quiet --project talos-testbed compute images delete talos-e2e
./google-cloud-sdk/bin/gcloud --quiet --project talos-testbed compute images create talos-e2e --source-uri gs://talos-e2e/talos-gce.tar.gz
