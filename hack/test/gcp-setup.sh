#!/bin/bash

set -eou pipefail

TMP=/tmp/e2e/gcp

mkdir -p ${TMP}

## Setup svc acct
echo $GCE_SVC_ACCT | base64 -d > ${TMP}/svc-acct.json

gcloud auth activate-service-account --key-file ${TMP}/svc-acct.json

## Push talos-gcp to storage bucket
gsutil cp ${ARTIFACTS}/gcp.tar.gz gs://talos-e2e/gcp-${SHA}.tar.gz

## Create image from talos-gcp
gcloud --quiet --project talos-testbed compute images delete talos-e2e-${SHA} || true ##Ignore error if image doesn't exist
gcloud --quiet --project talos-testbed compute images create talos-e2e-${SHA} --source-uri gs://talos-e2e/gcp-${SHA}.tar.gz

## Setup the cluster YAML.
sed -e "s/{{TAG}}/${SHA}/" ${PWD}/hack/test/manifests/gcp-cluster.yaml > ${TMP}/cluster.yaml
