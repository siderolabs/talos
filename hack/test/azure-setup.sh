#!/bin/bash

set -eou pipefail

STORAGE_ACCOUNT=talostesting
STORAGE_CONTAINER=talostesting
GROUP=talos
TMP=/tmp/e2e/azure

## Setup svc acct vars
mkdir -p ${TMP}
echo ${AZURE_SVC_ACCT} | base64 -d > ${TMP}/svc-acct.json
CLIENT_ID="$( cat ${TMP}/svc-acct.json | jq -r '.clientId' )"
CLIENT_SECRET="$( cat ${TMP}/svc-acct.json | jq -r '.clientSecret' )"
TENANT_ID="$( cat ${TMP}/svc-acct.json | jq -r '.tenantId' )"

## Untar image
tar -C ${TMP} -xf ${ARTIFACTS}/azure.tar.gz

## Login to azure
az login --service-principal --username ${CLIENT_ID} --password ${CLIENT_SECRET} --tenant ${TENANT_ID} > /dev/null

## Get connection string
AZURE_STORAGE_CONNECTION_STRING=$(az storage account show-connection-string -n ${STORAGE_ACCOUNT} -g ${GROUP} -o tsv)

## Push blob
AZURE_STORAGE_CONNECTION_STRING="${AZURE_STORAGE_CONNECTION_STRING}" az storage blob upload --container-name ${STORAGE_CONTAINER} -f ${TMP}/disk.vhd -n azure-${TAG}.vhd

## Delete image
az image delete --name talos-e2e-${TAG} -g ${GROUP}

## Create image
az image create --name talos-e2e-${TAG} --source https://${STORAGE_ACCOUNT}.blob.core.windows.net/${STORAGE_CONTAINER}/azure-${TAG}.vhd --os-type linux -g ${GROUP}

## Setup the cluster YAML.
sed "s/{{TAG}}/${TAG}/" ${PWD}/hack/test/manifests/azure-cluster.yaml > ${TMP}/cluster.yaml
