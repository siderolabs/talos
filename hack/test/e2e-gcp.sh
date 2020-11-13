#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

function setup {
  set +x
  echo ${GCE_SVC_ACCT} | base64 -d > ${TMP}/svc-acct.json
  gcloud auth activate-service-account --key-file ${TMP}/svc-acct.json
  set -x

  gsutil cp ${ARTIFACTS}/gcp-amd64.tar.gz gs://talos-e2e/gcp-${SHA}.tar.gz
  gcloud --quiet --project talos-testbed compute images delete talos-e2e-${SHA} || true
  gcloud --quiet --project talos-testbed compute images create talos-e2e-${SHA} --source-uri gs://talos-e2e/gcp-${SHA}.tar.gz
  sed -e "s/{{TAG}}/${SHA}/" ${PWD}/hack/test/capi/cluster-gcp.yaml > ${TMP}/cluster.yaml
}

setup
create_cluster_capi gcp
run_talos_integration_test
run_kubernetes_integration_test
