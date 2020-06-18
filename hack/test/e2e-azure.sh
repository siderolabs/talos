#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

function setup {
  AZURE_STORAGE_ACCOUNT=talostesting
  AZURE_STORAGE_CONTAINER=talostesting
  AZURE_GROUP=talos

  # Setup svc acct vars
  echo ${AZURE_SVC_ACCT} | base64 -d > ${TMP}/svc-acct.json
  AZURE_CLIENT_ID="$( cat ${TMP}/svc-acct.json | jq -j '.clientId' )"
  AZURE_CLIENT_SECRET="$( cat ${TMP}/svc-acct.json | jq -j '.clientSecret' )"
  AZURE_TENANT_ID="$( cat ${TMP}/svc-acct.json | jq -j '.tenantId' )"

  # Untar image
  tar -C ${TMP} -xf ${ARTIFACTS}/azure.tar.gz

  # Login to azure
  az login --service-principal --username ${AZURE_CLIENT_ID} --password ${AZURE_CLIENT_SECRET} --tenant ${AZURE_TENANT_ID} > /dev/null

  # Get connection string
  AZURE_STORAGE_CONNECTION_STRING=$(az storage account show-connection-string -n ${AZURE_STORAGE_ACCOUNT} -g ${AZURE_GROUP} -o tsv)

  # Push blob
  AZURE_STORAGE_CONNECTION_STRING="${AZURE_STORAGE_CONNECTION_STRING}" az storage blob upload --container-name ${AZURE_STORAGE_CONTAINER} -f ${TMP}/disk.vhd -n azure-${SHA}.vhd

  # Delete image
  az image delete --name talos-e2e-${SHA} -g ${AZURE_GROUP}

  # Create image
  az image create --name talos-e2e-${SHA} --source https://${AZURE_STORAGE_ACCOUNT}.blob.core.windows.net/${AZURE_STORAGE_CONTAINER}/azure-${SHA}.vhd --os-type linux -g ${AZURE_GROUP}

  # Setup the cluster YAML.
  sed -e "s/{{TAG}}/${SHA}/" \
      -e "s/{{AZURE_DUMMY_SSH_PUB}}/${AZURE_DUMMY_SSH_PUB}/" \
      ${PWD}/hack/test/capi/cluster-azure.yaml > ${TMP}/cluster.yaml
}

setup
create_cluster_capi azure
run_talos_integration_test
run_kubernetes_integration_test
