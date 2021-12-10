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
  export GCP_PROJECT=talos-testbed
  export GCP_REGION=us-central1
  export GCP_NETWORK=default
  export GCP_VM_SVC_ACCOUNT=e2e-tester@talos-testbed.iam.gserviceaccount.com


  ## Control plane vars
  export CONTROL_PLANE_MACHINE_COUNT=3
  export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-4
  export GCP_CONTROL_PLANE_VOL_SIZE=50
  export GCP_CONTROL_PLANE_IMAGE_ID=projects/${GCP_PROJECT}/global/images/talos-e2e-${SHA}

  ## Worker vars
  export WORKER_MACHINE_COUNT=3
  export GCP_NODE_MACHINE_TYPE=n1-standard-4
  export GCP_NODE_VOL_SIZE=50
  export GCP_NODE_IMAGE_ID=projects/${GCP_PROJECT}/global/images/talos-e2e-${SHA}

  ${CLUSTERCTL} generate cluster ${NAME_PREFIX} \
    --kubeconfig /tmp/e2e/docker/kubeconfig \
    --from https://github.com/talos-systems/cluster-api-templates/blob/v1beta1/gcp/standard/standard.yaml > ${TMP}/cluster.yaml
  
}

setup
create_cluster_capi gcp
run_talos_integration_test
run_kubernetes_integration_test
