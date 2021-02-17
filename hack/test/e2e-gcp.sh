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

  ## Cluster-wide vars
  export CLUSTER_NAME=${NAME_PREFIX}
  export PROJECT=talos-testbed
  export REGION=us-central1
  export NETWORK=default

  ## Control plane vars
  export CP_COUNT=3
  export CP_INSTANCE_TYPE=n1-standard-4
  export CP_VOL_SIZE=50
  export CP_IMAGE_ID=projects/${PROJECT}/global/images/talos-e2e-${SHA}

  ## Worker vars
  export WORKER_COUNT=3
  export WORKER_INSTANCE_TYPE=n1-standard-4
  export WORKER_VOL_SIZE=50
  export WORKER_IMAGE_ID=projects/${PROJECT}/global/images/talos-e2e-${SHA}

  ## TODO: update to talos-systems once merged
  ${CLUSTERCTL} config cluster ${NAME_PREFIX} \
    --kubeconfig /tmp/e2e/docker/kubeconfig \
    --from https://github.com/rsmitty/cluster-api-templates/blob/main/gcp/standard/standard.yaml > ${TMP}/cluster.yaml
  
}

setup
create_cluster_capi gcp
run_talos_integration_test
run_kubernetes_integration_test
