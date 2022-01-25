#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

function setup {
  AZURE_STORAGE_ACCOUNT=talostesting
  AZURE_STORAGE_CONTAINER=talostesting
  AZURE_GROUP=talos

  # Setup svc acct vars
  set +x
  echo ${AZURE_SVC_ACCT} | base64 -d > ${TMP}/svc-acct.json
  AZURE_CLIENT_ID="$( cat ${TMP}/svc-acct.json | jq -r '.clientId' )"
  AZURE_CLIENT_SECRET="$( cat ${TMP}/svc-acct.json | jq -r '.clientSecret' )"
  AZURE_TENANT_ID="$( cat ${TMP}/svc-acct.json | jq -r '.tenantId' )"

  # Login to azure
  az login --service-principal --username ${AZURE_CLIENT_ID} --password ${AZURE_CLIENT_SECRET} --tenant ${AZURE_TENANT_ID} > /dev/null
  set -x

  # Untar image
  tar -C ${TMP} -xf ${ARTIFACTS}/azure-amd64.tar.gz

  # Get connection string
  AZURE_STORAGE_CONNECTION_STRING=$(az storage account show-connection-string -n ${AZURE_STORAGE_ACCOUNT} -g ${AZURE_GROUP} -o tsv)

  # Push blob
  AZURE_STORAGE_CONNECTION_STRING="${AZURE_STORAGE_CONNECTION_STRING}" az storage blob upload --container-name ${AZURE_STORAGE_CONTAINER} -f ${TMP}/disk.vhd -n azure-${TAG}.vhd

  # Delete image
  az image delete --name talos-e2e-${TAG} -g ${AZURE_GROUP}

  # Create image
  az image create --name talos-e2e-${TAG} --source https://${AZURE_STORAGE_ACCOUNT}.blob.core.windows.net/${AZURE_STORAGE_CONTAINER}/azure-${TAG}.vhd --os-type linux -g ${AZURE_GROUP}

  # Setup the cluster YAML.
  sed "s/{{TAG}}/${TAG}/" ${PWD}/hack/test/manifests/azure-cluster.yaml > ${TMP}/cluster.yaml
}

setup
create_cluster_capi azure
run_talos_integration_test
run_kubernetes_integration_test
